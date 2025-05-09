// The namesrv package implements the realm's root name server using
// fsetcd.
package namesrv

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/namesrv/leaderetcd"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	spprotosrv "sigmaos/spproto/srv"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
)

type Named struct {
	*sigmaclnt.SigmaClnt
	*sigmasrv.SigmaSrv
	mu     sync.Mutex
	fs     *fsetcd.FsEtcd
	elect  *leaderetcd.Election
	job    string
	realm  sp.Trealm
	delay  int64
	sess   *fsetcd.Session
	pstats *fsetcd.PstatInode
	ephch  chan path.Tpathname
}

func newNamed(realm sp.Trealm) *Named {
	nd := &Named{
		realm: realm,
		ephch: make(chan path.Tpathname),
	}
	return nd
}

func toGiB(nbyte uint64) float64 {
	return float64(nbyte) / float64(1<<30)
}

//	go func() {
//		for {
//			time.Sleep(1000 * time.Millisecond)
//			var ms runtime.MemStats
//			runtime.ReadMemStats(&ms)
//			db.DPrintf(db.ALWAYS, "Num goroutines (%v) HeapLiveBytes:(%.3f) TotalHeapAllocCum:(%3f) MaxHeapSizeEver:(%.3f) HeapNotReleasedToSys:(%.3f) HeapReleasedToSys:(%.3f) StackInuse:(%.3f) StackReqeuestedFromSys:(%.3f) SysAllocated:(%.3f)", runtime.NumGoroutine(), toGiB(ms.HeapAlloc), toGiB(ms.TotalAlloc), toGiB(ms.HeapSys), toGiB(ms.HeapIdle), toGiB(ms.HeapReleased), toGiB(ms.StackInuse), toGiB(ms.StackSys), toGiB(ms.Sys))
//		}
//	}()

func Run(args []string) error {
	pe := proc.GetProcEnv()
	db.DPrintf(db.NAMED_LDR, "named start: %v cfg: %v", args, pe)
	if len(args) != 2 {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}

	nd := newNamed(sp.Trealm(args[1]))
	p, err := perf.NewPerf(pe, perf.NAMED)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	sc, err := sigmaclnt.NewSigmaClntFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
		return err
	}
	nd.SigmaClnt = sc

	// Manually mount some directories from the root named, to which the root
	// named explicitly allows attaches
	rootEP, err := sc.GetNamedEndpointRealm(sp.ROOTREALM)
	if err != nil {
		db.DFatalf("Error get named EP: %v", err)
	}
	if err := sc.MountTree(rootEP, rpc.RPC, rpc.RPC); err != nil {
		db.DFatalf("Err MountTree: ep %v err %v", rootEP, err)
	}
	if nd.realm != sp.ROOTREALM {
		if err := sc.MountTree(rootEP, sp.REALMREL, sp.REALM); err != nil {
			db.DFatalf("Err MountTree realm: ep %v err %v", rootEP, err)
		}
		if err := sc.MountTree(rootEP, sp.REALMSREL, sp.REALMS); err != nil {
			db.DFatalf("Err MountTree realm: ep %v err %v", rootEP, err)
		}
		if err := sc.MountTree(rootEP, rpc.RPC, filepath.Join(sp.REALMS, rpc.RPC)); err != nil {
			db.DFatalf("Err MountTree realm: ep %v err %v", rootEP, err)
		}
		// Must manually mount scheduler dirs, since they will be automatically
		// scanned by msched-/procq-/lcsched- clnts as soon as the procclnt is
		// created, but this named won't have posted its endpoint in the namespace
		// yet, so root named resolution will fail.
		if err := sc.MountTree(rootEP, sp.MSCHEDREL, sp.MSCHED); err != nil {
			db.DFatalf("Err MountTree msched: ep %v err %v", rootEP, err)
		}
		if err := sc.MountTree(rootEP, sp.BESCHEDREL, sp.BESCHED); err != nil {
			db.DFatalf("Err MountTree procq: ep %v err %v", rootEP, err)
		}
		if err := sc.MountTree(rootEP, sp.LCSCHEDREL, sp.LCSCHED); err != nil {
			db.DFatalf("Err MountTree lcsched: ep %v err %v", rootEP, err)
		}
	}

	// Now that the scheduler dirs are mounted, create a procclnt
	if err := nd.SigmaClnt.NewProcClnt(); err != nil {
		db.DFatalf("Error make procclnt: %v", err)
	}

	if err := nd.Started(); err != nil {
		db.DFatalf("Error Started: %v", err)
	}

	ch := make(chan struct{})
	go nd.waitExit(ch)

	db.DPrintf(db.NAMED_LDR, "started %v %v", pe.GetPID(), nd.realm)

	if err := nd.startLeader(); err != nil {
		db.DPrintf(db.NAMED_LDR, "%v: startLeader %v err %v", pe.GetPID(), nd.realm, err)
		return err
	}
	defer nd.fs.Close()

	go func() {
		<-nd.sess.Done()
		db.DPrintf(db.NAMED_LDR, "session expired delay %v", nd.delay)
		time.Sleep(time.Duration(nd.delay) * time.Millisecond)
		nd.resign()
	}()

	ep, err := nd.newSrv()
	if err != nil {
		db.DFatalf("Error newSrv %v", err)
	}

	nd.SigmaSrv.Mount(sp.PSTATSD, nd.pstats)

	db.DPrintf(db.NAMED_LDR, "newSrv %v -> ep %v", nd.realm, ep)

	pn := sp.NAMED
	if nd.realm == sp.ROOTREALM {
		// Allow connections from all realms, so that realms can mount the kernel
		// service union directories
		nd.GetDialProxyClnt().AllowConnectionsFromAllRealms()
		db.DPrintf(db.ALWAYS, "SetRootNamed %v ep %v", nd.realm, ep)
		if err := nd.fs.SetRootNamed(ep); err != nil {
			db.DFatalf("SetNamed: %v", err)
		}
	} else {
		pn = path.MarkResolve(filepath.Join(sp.REALMS, nd.realm.String()))
		db.DPrintf(db.ALWAYS, "NewEndpointSymlink %v %v lid %v", nd.realm, pn, nd.sess.Lease())
		li, err := sc.LeaseClnt.AskLease(pn, fsetcd.LeaseTTL)
		if err != nil {
			db.DFatalf("Error AskLease %v: %v", pn, err)
		}
		li.KeepExtending()

		if err := nd.MkLeasedEndpoint(pn, ep, li.Lease()); err != nil {
			db.DPrintf(db.ERROR, "MkEndpointFile %v at %v err %v", nd.realm, pn, err)
			db.DFatalf("MkEndpointFile %v at %v err %v", nd.realm, pn, err)
			return err
		}
		db.DPrintf(db.NAMED_LDR, "[%v] named endpoint %v", nd.realm, ep)
	}

	nd.getRoot(path.MarkResolve(pn))

	if err := nd.CreateLeaderFile(filepath.Join(sp.NAME, nd.elect.Key()), nil, sp.TleaseId(nd.sess.Lease()), nd.elect.Fence()); err != nil {
		db.DPrintf(db.NAMED, "CreateElectionInfo %v err %v", nd.elect.Key(), err)
	}

	db.DPrintf(db.NAMED_LDR, "Created Leader file %v ", nd.elect.Key())

	if err := nd.warmCache(); err != nil {
		db.DFatalf("warmCache err %v", err)
	}

	crash.Failer(nd.FsLib, crash.NAMED_CRASH, func(e crash.Tevent) {
		crash.Crash()
	})

	crash.Failer(nd.FsLib, crash.NAMED_PARTITION, func(e crash.Tevent) {
		if nd.delay == 0 {
			nd.delay = e.Delay
			nd.sess.Orphan()
		}
	})

	<-ch

	db.DPrintf(db.ALWAYS, "named done %v %v", nd.realm, ep)

	if err := nd.resign(); err != nil {
		db.DPrintf(db.NAMED_LDR, "resign %v err %v", pe.GetPID(), err)
	}

	nd.SigmaSrv.SrvExit(proc.NewStatus(proc.StatusEvicted))

	return nil
}

func (nd *Named) newSrv() (*sp.Tendpoint, error) {
	ip := sp.NO_IP
	root := RootDir(nd.fs, nd.realm)
	var addr *sp.Taddr
	var aaf spprotosrv.AttachAuthF
	// If this is a root named, don't do
	// anything special.
	if nd.realm == sp.ROOTREALM {
		addr = sp.NewTaddr(ip, sp.NO_PORT)
		// Allow all realms to attach to dirs mounted from the root named, as well as RPC dir, since it is needed to take out leases
		allowedDirs := []string{rpc.RPC, sp.REALMSREL}
		for s, _ := range sp.RootNamedMountedDirs {
			allowedDirs = append(allowedDirs, s)
		}
		aaf = spprotosrv.AttachAllowAllPrincipalsSelectPaths(allowedDirs)
	} else {
		addr = sp.NewTaddr(ip, sp.NO_PORT)
		aaf = spprotosrv.AttachAllowAllToAll
	}
	ssrv, err := sigmasrv.NewSigmaSrvRootClntAuthFn(root, addr, "", nd.SigmaClnt, aaf)
	if err != nil {
		return nil, fmt.Errorf("NewSigmaSrvRootClnt err: %v", err)
	}

	if err := ssrv.MountRPCSrv(newLeaseSrv(nd.fs)); err != nil {
		return nil, err
	}
	nd.SigmaSrv = ssrv

	// now we have a SigmaSrv read from ephch
	go nd.watchLeased()

	ep := nd.GetEndpoint()
	db.DPrintf(db.NAMED_LDR, "newSrv %v %v %v %v %v", nd.realm, addr, ssrv.GetEndpoint(), nd.elect.Key(), ep)
	return ep, nil
}

func (nd *Named) attach(cid sp.TclntId) {
	db.DPrintf(db.NAMED, "named: attach %v", cid)
	// nd.fs.Recover(cid)
}

func (nd *Named) detach(cid sp.TclntId) {
	db.DPrintf(db.NAMED, "named: detach %v", cid)
	// nd.fs.Detach(cid)
}

func (nd *Named) resign() error {
	db.DPrintf(db.NAMED_LDR, "%v resign", nd.realm)
	if err := nd.fs.StopWatch(); err != nil {
		return err
	}
	if err := nd.SigmaPSrv.StopServing(); err != nil {
		return err
	}
	return nd.elect.Resign()
}

func (nd *Named) getRoot(pn string) error {
	sts, err := nd.GetDir(pn)
	if err != nil {
		db.DPrintf(db.NAMED, "getdir %v err %v", pn, err)
		return err
	}
	db.DPrintf(db.NAMED, "getdir %v sts %v", pn, sp.Names(sts))
	return nil
}

func (nd *Named) waitExit(ch chan struct{}) {
	var i int = 0
	for ; ; i++ {
		err := nd.WaitEvict(nd.ProcEnv().GetPID())
		if err == nil {
			db.DPrintf(db.ALWAYS, "candidate %v %v evicted", nd.realm, nd.ProcEnv().GetPID().String())
			ch <- struct{}{}
			break
		}
		if i > sp.Conf.Path.MAX_RESOLVE_RETRY {
			db.DPrintf(db.ALWAYS, "candidate %v %v err evict giving up!", nd.realm, nd.ProcEnv().GetPID().String())
			ch <- struct{}{}
			break
		}
		db.DPrintf(db.NAMED, "Error WaitEvict: %v", err)
		time.Sleep(sp.Conf.Path.RESOLVE_TIMEOUT)
		continue
	}
}

func (nd *Named) watchLeased() {
	for pn := range nd.ephch {
		nd.SigmaSrv.Notify(pn)
	}
}

// XXX same as initRootDir?
var warmRootDir = []string{sp.BOOT, sp.KPIDS, sp.MEMFS, sp.LCSCHED, sp.BESCHED, sp.MSCHED, sp.UX, sp.S3, sp.DB, sp.MONGO, sp.REALM, sp.CHUNKD}

func (nd *Named) warmCache() error {
	for _, n := range warmRootDir {
		if sts, err := nd.GetDir(n); err == nil {
			db.DPrintf(db.NAMED, "Warm cache %v: %v", n, sp.Names(sts))
		} else {
			db.DPrintf(db.NAMED, "Warm cache %v err %v", n, err)
		}
	}
	return nil
}
