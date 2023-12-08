package named

import (
	"fmt"
	"path"
	"strconv"
	"sync"
	"time"

	//	"runtime"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslibsrv"
	"sigmaos/leaderetcd"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/portclnt"
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
	mu    sync.Mutex
	fs    *fsetcd.FsEtcd
	elect *leaderetcd.Election
	job   string
	realm sp.Trealm
	crash int
	sess  *fsetcd.Session
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

	pcfg := proc.GetProcEnv()
	db.DPrintf(db.NAMED, "named started: %v cfg: %v", args, pcfg)
	if len(args) != 3 {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}
	nd := &Named{}
	nd.realm = sp.Trealm(args[1])
	crashing, err := strconv.Atoi(args[2])
	if err != nil {
		return fmt.Errorf("%v: crash %v isn't int", args[0], args[2])
	}
	nd.crash = crashing

	p, err := perf.NewPerf(pcfg, perf.NAMED)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	sc, err := sigmaclnt.NewSigmaClnt(pcfg)
	if err != nil {
		return err
	}
	nd.SigmaClnt = sc

	pn := path.Join(sp.REALMS, nd.realm.String()) + ".sem"
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

	db.DPrintf(db.NAMED, "started %v %v", pcfg.GetPID(), nd.realm)

	if err := nd.startLeader(); err != nil {
		db.DPrintf(db.NAMED, "%v: startLeader %v err %v\n", pcfg.GetPID(), nd.realm, err)
		return err
	}
	defer nd.fs.Close()

	mnt, err := nd.newSrv()
	if err != nil {
		db.DFatalf("Error newSrv %v\n", err)
	}

	db.DPrintf(db.NAMED, "newSrv %v mnt %v", nd.realm, mnt)

	pn = sp.NAMED
	if nd.realm == sp.ROOTREALM {
		db.DPrintf(db.ALWAYS, "SetRootNamed %v mnt %v\n", nd.realm, mnt)
		if err := nd.fs.SetRootNamed(mnt); err != nil {
			db.DFatalf("SetNamed: %v", err)
		}
	} else {
		// note: the named proc runs in rootrealm; maybe change it XXX
		pn = path.Join(sp.REALMS, nd.realm.String())
		db.DPrintf(db.ALWAYS, "NewMountSymlink %v %v lid %v\n", nd.realm, pn, nd.sess.Lease())
		if err := nd.NewMountSymlink(pn, mnt, nd.sess.Lease()); err != nil {
			db.DPrintf(db.NAMED, "mount %v at %v err %v\n", nd.realm, pn, err)
			return err
		}

		// Signal realmd we are ready
		if err := sem.Up(); err != nil {
			db.DPrintf(db.NAMED, "%v sem up %v err %v\n", nd.realm, sem.String(), err)
			return err
		}
	}

	nd.getRoot(pn + "/")

	if err := nd.CreateLeaderFile(path.Join(sp.NAME, nd.elect.Key()), nil, sp.TleaseId(nd.sess.Lease()), nd.elect.Fence()); err != nil {
		db.DPrintf(db.NAMED, "CreateElectionInfo %v err %v\n", nd.elect.Key(), err)
	}

	db.DPrintf(db.NAMED, "Created Leader file %v ", nd.elect.Key())

	if nd.crash > 0 {
		crash.Crasher(nd.SigmaClnt.FsLib)
	}

	<-ch

	db.DPrintf(db.NAMED, "%v: named done %v %v\n", pcfg.GetPID(), nd.realm, mnt)

	if err := nd.resign(); err != nil {
		db.DPrintf(db.NAMED, "resign %v err %v\n", pcfg.GetPID(), err)
	}

	nd.SigmaSrv.SrvExit(proc.NewStatus(proc.StatusEvicted))

	return nil
}

func (nd *Named) newSrv() (sp.Tmount, error) {
	ip := nd.ProcEnv().GetLocalIP()
	root := rootDir(nd.fs, nd.realm)
	var pi portclnt.PortInfo
	if nd.realm == sp.ROOTREALM || nd.ProcEnv().GetNet() == sp.ROOTREALM.String() {
		ip = ip + ":0"
	} else {
		_, pi0, err := portclnt.NewPortClntPort(nd.SigmaClnt.FsLib)
		if err != nil {
			return sp.NullMount(), err
		}
		pi = pi0
		ip = ":" + pi.Pb.RealmPort.String()
	}
	srv := fslibsrv.BootSrv(nd.ProcEnv(), root, ip, nd.attach, nd.detach, nil)
	if srv == nil {
		return sp.NullMount(), fmt.Errorf("BootSrv err")
	}

	ssrv := sigmasrv.NewSigmaSrvSess(srv, nd.SigmaClnt)
	if err := ssrv.MountRPCSrv(newLeaseSrv(nd.fs)); err != nil {
		return sp.NullMount(), err
	}
	nd.SigmaSrv = ssrv

	mnt := sp.NewMountServer(nd.MyAddr())
	if nd.realm != sp.ROOTREALM {
		mnt = port.NewPublicMount(pi.Hip, pi.Pb, nd.ProcEnv().GetNet(), nd.MyAddr())
	}

	db.DPrintf(db.NAMED, "newSrv %v %v %v %v %v\n", nd.realm, ip, srv.MyAddr(), nd.elect.Key(), mnt)

	return mnt, nil
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
	if err := nd.SessSrv.StopServing(); err != nil {
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
		if err != nil {
			db.DPrintf(db.NAMED, "Error WaitEvict: %v", err)
			time.Sleep(time.Second)
			continue
		}
		db.DPrintf(db.NAMED, "candidate %v %v evicted\n", nd.realm, nd.ProcEnv().GetPID().String())
		ch <- struct{}{}
	}
}
