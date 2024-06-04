package namesrv

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/leaderetcd"
	"sigmaos/path"
	"sigmaos/perf"
	"sigmaos/proc"
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
	ephch  chan path.Tpathname
}

func newNamed(realm sp.Trealm) *Named {
	nd := &Named{realm: realm, ephch: make(chan path.Tpathname)}
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

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return err
	}
	nd.SigmaClnt = sc

	pn := filepath.Join(sp.REALMS, nd.realm.String()) + ".sem"
	sem := semclnt.NewSemClnt(nd.FsLib, pn)
	if nd.realm != sp.ROOTREALM {
		// create semaphore to signal realmd when we are the leader
		// and ready to serve requests.  realmd downs this semaphore.
		li, err := sc.LeaseClnt.AskLease(pn, fsetcd.LeaseTTL)
		if err != nil {
			return err
		}
		li.KeepExtending()
		if err := sem.InitLease(0777, li.Lease()); err != nil {
			return err
		}
	}

	nd.Started()
	ch := make(chan struct{})
	go nd.waitExit(ch)

	db.DPrintf(db.NAMED, "started %v %v", pe.GetPID(), nd.realm)

	if err := nd.startLeader(); err != nil {
		db.DPrintf(db.NAMED, "%v: startLeader %v err %v\n", pe.GetPID(), nd.realm, err)
		return err
	}
	defer nd.fs.Close()

	ep, err := nd.newSrv()
	if err != nil {
		db.DFatalf("Error newSrv %v\n", err)
	}

	db.DPrintf(db.NAMED, "newSrv %v ep %v", nd.realm, ep)

	pn = sp.NAMED
	if nd.realm == sp.ROOTREALM {
		db.DPrintf(db.ALWAYS, "SetRootNamed %v ep %v\n", nd.realm, ep)
		if err := nd.fs.SetRootNamed(ep); err != nil {
			db.DFatalf("SetNamed: %v", err)
		}
	} else {
		// note: the named proc runs in rootrealm; maybe change it XXX
		pn = filepath.Join(sp.REALMS, nd.realm.String())
		db.DPrintf(db.ALWAYS, "NewEndpointSymlink %v %v lid %v\n", nd.realm, pn, nd.sess.Lease())
		if err := nd.MkLeasedEndpoint(pn, ep, nd.sess.Lease()); err != nil {
			db.DPrintf(db.NAMED, "MkEndpointFile %v at %v err %v\n", nd.realm, pn, err)
			return err
		}

		// Signal realmd we are ready
		if err := sem.Up(); err != nil {
			db.DPrintf(db.NAMED, "%v sem up %v err %v\n", nd.realm, sem.String(), err)
			return err
		}
	}

	nd.getRoot(pn + "/")

	if err := nd.CreateLeaderFile(filepath.Join(sp.NAME, nd.elect.Key()), nil, sp.TleaseId(nd.sess.Lease()), nd.elect.Fence()); err != nil {
		db.DPrintf(db.NAMED, "CreateElectionInfo %v err %v\n", nd.elect.Key(), err)
	}

	db.DPrintf(db.NAMED, "Created Leader file %v ", nd.elect.Key())

	if nd.crash > 0 {
		crash.Crasher(nd.SigmaClnt.FsLib)
	}

	<-ch

	db.DPrintf(db.ALWAYS, "%v: named done %v %v\n", pe.GetPID(), nd.realm, ep)

	if err := nd.resign(); err != nil {
		db.DPrintf(db.NAMED, "resign %v err %v\n", pe.GetPID(), err)
	}

	nd.SigmaSrv.SrvExit(proc.NewStatus(proc.StatusEvicted))

	return nil
}

func (nd *Named) newSrv() (*sp.Tendpoint, error) {
	ip := sp.NO_IP
	root := rootDir(nd.fs, nd.realm)
	var addr *sp.Taddr
	// XXX need special handling with overlays?
	//	var pi portclnt.PortInfo
	//	if nd.realm == sp.ROOTREALM || nd.ProcEnv().GetNet() == sp.ROOTREALM.String() {
	addr = sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, sp.NO_PORT)
	//	} else {
	//		_, pi0, err := portclnt.NewPortClntPort(nd.SigmaClnt.FsLib)
	//		if err != nil {
	//			return nil, err
	//		}
	//		pi = pi0
	//		addr = sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, pi.PBinding.RealmPort)
	//	}
	ssrv, err := sigmasrv.NewSigmaSrvRootClnt(root, addr, "", nd.SigmaClnt)
	if err != nil {
		return nil, fmt.Errorf("NewSigmaSrvRootClnt err: %v", err)
	}

	if err := ssrv.MountRPCSrv(newLeaseSrv(nd.fs)); err != nil {
		return nil, err
	}
	nd.SigmaSrv = ssrv

	ep := nd.GetEndpoint()
	// XXX need public endpoint?
	//	if nd.realm != sp.ROOTREALM {
	//		ep = port.NewPublicEndpoint(pi.HostIP, pi.PBinding, nd.ProcEnv().GetNet(), nd.GetEndpoint())
	//	}
	// ep.SetType(sp.INTERNAL_EP)
	db.DPrintf(db.NAMED, "newSrv %v %v %v %v %v\n", nd.realm, addr, ssrv.GetEndpoint(), nd.elect.Key(), ep)
	return ep, nil
}

func (nd *Named) attach(cid sp.TclntId) {
	db.DPrintf(db.NAMED, "named: attach %v\n", cid)
	// nd.fs.Recover(cid)
}

func (nd *Named) detach(cid sp.TclntId) {
	db.DPrintf(db.NAMED, "named: detach %v\n", cid)
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
		db.DPrintf(db.NAMED, "getdir %v err %v\n", pn, err)
		return err
	}
	db.DPrintf(db.NAMED, "getdir %v sts %v\n", pn, sp.Names(sts))
	return nil
}

func (nd *Named) waitExit(ch chan struct{}) {
	for {
		err := nd.WaitEvict(nd.ProcEnv().GetPID())
		if err == nil {
			db.DPrintf(db.ALWAYS, "candidate %v %v evicted\n", nd.realm, nd.ProcEnv().GetPID().String())
			ch <- struct{}{}
			break
		}
		db.DPrintf(db.NAMED, "Error WaitEvict: %v", err)
		time.Sleep(time.Second)
		continue
	}
}

func (nd *Named) watchEphemeral() {
	for pn := range nd.ephch {
		nd.SigmaSrv.Notify(pn)
	}
}
