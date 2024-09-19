package namesrv

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/leaderetcd"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/netproxyclnt"
	"sigmaos/path"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/protsrv"
	"sigmaos/rpc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

// Named implements fs/fs.go using fsetcd.  It assumes that its caller
// (protsrv) holds read/write locks.

type Named struct {
	*sigmaclnt.SigmaClnt
	*sigmasrv.SigmaSrv
	mu     sync.Mutex
	fs     *fsetcd.FsEtcd
	elect  *leaderetcd.Election
	job    string
	realm  sp.Trealm
	crash  int
	sess   *fsetcd.Session
	signer sp.Tsigner
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

func Run(args []string) error {
	//	go func() {
	//		for {
	//			time.Sleep(1000 * time.Millisecond)
	//			var ms runtime.MemStats
	//			runtime.ReadMemStats(&ms)
	//			db.DPrintf(db.ALWAYS, "Num goroutines (%v) HeapLiveBytes:(%.3f) TotalHeapAllocCum:(%3f) MaxHeapSizeEver:(%.3f) HeapNotReleasedToSys:(%.3f) HeapReleasedToSys:(%.3f) StackInuse:(%.3f) StackReqeuestedFromSys:(%.3f) SysAllocated:(%.3f)", runtime.NumGoroutine(), toGiB(ms.HeapAlloc), toGiB(ms.TotalAlloc), toGiB(ms.HeapSys), toGiB(ms.HeapIdle), toGiB(ms.HeapReleased), toGiB(ms.StackInuse), toGiB(ms.StackSys), toGiB(ms.Sys))
	//		}
	//	}()

	pe := proc.GetProcEnv()
	db.DPrintf(db.NAMED, "named started: %v cfg: %v", args, pe)
	if len(args) != 3 {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}

	nd := newNamed(sp.Trealm(args[1]))
	nd.signer = sp.Tsigner(pe.GetPID())
	crashing, err := strconv.Atoi(args[2])
	if err != nil {
		return fmt.Errorf("%v: crash %v isn't int", args[0], args[1])
	}
	nd.crash = crashing

	p, err := perf.NewPerf(pe, perf.NAMED)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	sc, err := sigmaclnt.NewSigmaClntFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
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
		// Must manually mount scheduler dirs, since they will be automatically
		// scanned by schedd-/procq-/lcsched- clnts as soon as the procclnt is
		// created, but this named won't have posted its endpoint in the namespace
		// yet, so root named resolution will fail.
		if err := sc.MountTree(rootEP, sp.SCHEDDREL, sp.SCHEDD); err != nil {
			db.DFatalf("Err MountTree schedd: ep %v err %v", rootEP, err)
		}
		if err := sc.MountTree(rootEP, sp.PROCQREL, sp.PROCQ); err != nil {
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

	pn := filepath.Join(sp.REALMS, nd.realm.String()) + ".sem"
	sem := semclnt.NewSemClnt(nd.FsLib, pn)
	if nd.realm != sp.ROOTREALM {
		// create semaphore to signal realmd when we are the leader
		// and ready to serve requests.  realmd downs this semaphore.
		li, err := sc.LeaseClnt.AskLease(pn, fsetcd.LeaseTTL)
		if err != nil {
			db.DFatalf("Error AskLease: %v", err)
			return err
		}
		li.KeepExtending()
		if err := sem.InitLease(0777, li.Lease()); err != nil {
			db.DFatalf("Error InitLease: %v", err)
			return err
		}
	}

	if err := nd.Started(); err != nil {
		db.DFatalf("Error Started: %v", err)
	}

	ch := make(chan struct{})
	go nd.waitExit(ch)

	db.DPrintf(db.NAMED, "started %v %v", pe.GetPID(), nd.realm)

	if err := nd.startLeader(); err != nil {
		db.DPrintf(db.NAMED, "%v: startLeader %v err %v", pe.GetPID(), nd.realm, err)
		return err
	}
	defer nd.fs.Close()

	ep, err := nd.newSrv()
	if err != nil {
		db.DFatalf("Error newSrv %v", err)
	}

	nd.SigmaSrv.Mount(sp.PSTATSD, nd.pstats)

	db.DPrintf(db.NAMED, "newSrv %v ep %v", nd.realm, ep)

	pn = sp.NAMED
	if nd.realm == sp.ROOTREALM {
		// Allow connections from all realms, so that realms can mount the kernel
		// service union directories
		nd.GetNetProxyClnt().AllowConnectionsFromAllRealms()
		db.DPrintf(db.ALWAYS, "SetRootNamed %v ep %v", nd.realm, ep)
		if err := nd.fs.SetRootNamed(ep); err != nil {
			db.DFatalf("SetNamed: %v", err)
		}
	} else {
		pn = filepath.Join(sp.REALMS, nd.realm.String())
		db.DPrintf(db.ALWAYS, "NewEndpointSymlink %v %v lid %v", nd.realm, pn, nd.sess.Lease())
		if err := nd.MkLeasedEndpoint(pn, ep, nd.sess.Lease()); err != nil {
			db.DPrintf(db.NAMED, "MkEndpointFile %v at %v err %v", nd.realm, pn, err)
			return err
		}
		db.DPrintf(db.NAMED, "[%v] named endpoint %v", nd.realm, ep)

		// Signal realmd we are ready
		if err := sem.Up(); err != nil {
			db.DPrintf(db.NAMED, "%v sem up %v err %v", nd.realm, sem.String(), err)
			return err
		}
	}

	nd.getRoot(pn + "/")

	if err := nd.CreateLeaderFile(filepath.Join(sp.NAME, nd.elect.Key()), nil, sp.TleaseId(nd.sess.Lease()), nd.elect.Fence()); err != nil {
		db.DPrintf(db.NAMED, "CreateElectionInfo %v err %v", nd.elect.Key(), err)
	}

	db.DPrintf(db.NAMED, "Created Leader file %v ", nd.elect.Key())

	if err := nd.warmCache(); err != nil {
		db.DFatalf("warmCache err %v", err)
	}

	if nd.crash > 0 {
		crash.Crasher(nd.SigmaClnt.FsLib)
	}

	<-ch

	db.DPrintf(db.ALWAYS, "named done %v %v", nd.realm, ep)

	if err := nd.resign(); err != nil {
		db.DPrintf(db.NAMED, "resign %v err %v", pe.GetPID(), err)
	}

	nd.SigmaSrv.SrvExit(proc.NewStatus(proc.StatusEvicted))

	return nil
}

func (nd *Named) newSrv() (*sp.Tendpoint, error) {
	ip := sp.NO_IP
	root := rootDir(nd.fs, nd.realm)
	var addr *sp.Taddr
	var aaf protsrv.AttachAuthF
	// If this is a root named, or we are running without overlays, don't do
	// anything special.
	if nd.realm == sp.ROOTREALM || !nd.ProcEnv().GetOverlays() {
		addr = sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, sp.NO_PORT)
		// Allow all realms to attach to dirs mounted from the root named, as well as RPC dir, since it is needed to take out leases
		allowedDirs := []string{rpc.RPC}
		for s, _ := range sp.RootNamedMountedDirs {
			allowedDirs = append(allowedDirs, s)
		}
		aaf = protsrv.AttachAllowAllPrincipalsSelectPaths(allowedDirs)
	} else {
		db.DPrintf(db.NAMED, "[%v] Listeing on overlay public port: %v:%v", nd.realm, nd.ProcEnv().GetOuterContainerIP(), port.PUBLIC_NAMED_PORT)
		addr = sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, port.PUBLIC_NAMED_PORT)
		aaf = protsrv.AttachAllowAllToAll
	}
	ssrv, err := sigmasrv.NewSigmaSrvRootClntAuthFn(root, addr, "", nd.SigmaClnt, aaf)
	if err != nil {
		return nil, fmt.Errorf("NewSigmaSrvRootClnt err: %v", err)
	}

	if err := ssrv.MountRPCSrv(newLeaseSrv(nd.fs)); err != nil {
		return nil, err
	}
	nd.SigmaSrv = ssrv

	ep := nd.GetEndpoint()
	// If running with overlays, and this isn't the root named, fix up the
	// endpoint.
	if nd.realm != sp.ROOTREALM && nd.ProcEnv().GetOverlays() {
		pm, err := port.GetPublicPortBinding(nd.FsLib, sp.PUBLIC_NAMED_PORT)
		if err != nil {
			db.DFatalf("Error get port binding: %v", err)
		}
		// Fix up the endpoint to use the public port and IP address
		ep.Addrs()[0].IPStr = nd.ProcEnv().GetOuterContainerIP().String()
		ep.Addrs()[0].PortInt = uint32(pm.HostPort)
	}
	db.DPrintf(db.NAMED, "newSrv %v %v %v %v %v", nd.realm, addr, ssrv.GetEndpoint(), nd.elect.Key(), ep)
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
	for {
		err := nd.WaitEvict(nd.ProcEnv().GetPID())
		if err == nil {
			db.DPrintf(db.ALWAYS, "candidate %v %v evicted", nd.realm, nd.ProcEnv().GetPID().String())
			ch <- struct{}{}
			break
		}
		db.DPrintf(db.NAMED, "Error WaitEvict: %v", err)
		time.Sleep(time.Second)
		continue
	}
}

func (nd *Named) watchLeased() {
	for pn := range nd.ephch {
		nd.SigmaSrv.Notify(pn)
	}
}

// XXX same as initRootDir?
var warmRootDir = []string{sp.BOOT, sp.KPIDS, sp.MEMFS, sp.LCSCHED, sp.PROCQ, sp.SCHEDD, sp.UX, sp.S3, sp.DB, sp.MONGO, sp.REALM, sp.CHUNKD}

func (nd *Named) warmCache() error {
	for _, n := range warmRootDir {
		if sts, err := nd.GetDir(n); err == nil {
			db.DPrintf(db.TEST, "Warm cache %v: %v", n, sp.Names(sts))
		}
	}
	return nil
}
