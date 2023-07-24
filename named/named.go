package named

import (
	"fmt"
	"path"
	"strconv"
	"sync"
	"time"

	"sigmaos/container"
	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslibsrv"
	"sigmaos/leaderetcd"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

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

func Run(args []string) error {
	db.DPrintf(db.NAMED, "%v: %v\n", proc.GetPid(), args)
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

	uname := sp.Tuname(proc.GetPid().String())
	sc, err := sigmaclnt.MkSigmaClnt(uname)
	if err != nil {
		return err
	}
	nd.SigmaClnt = sc

	pn := path.Join(sp.REALMS, nd.realm.String()) + ".sem"
	sem := semclnt.MakeSemClnt(nd.FsLib, pn)
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

	db.DPrintf(db.NAMED, "started %v %v %v\n", proc.GetPid(), nd.realm, proc.GetRealm())

	if err := nd.startLeader(); err != nil {
		db.DPrintf(db.NAMED, "%v: startLeader %v err %v\n", proc.GetPid(), nd.realm, err)
		return err
	}
	defer nd.fs.Close()

	if err := nd.mkSrv(); err != nil {
		db.DFatalf("Error mkSrv %v\n", err)
	}

	mnt := sp.MkMountServer(nd.MyAddr())
	pn = sp.NAMED
	if nd.realm == sp.ROOTREALM {
		db.DPrintf(db.ALWAYS, "SetRootNamed %v mnt %v\n", nd.realm, mnt)
		if err := nd.fs.SetRootNamed(mnt); err != nil {
			db.DFatalf("SetNamed: %v", err)
		}
	} else {
		// note: the named proc runs in rootrealm; maybe change it XXX
		pn = path.Join(sp.REALMS, nd.realm.String())
		db.DPrintf(db.ALWAYS, "MkMountSymlink %v %v lid %v\n", nd.realm, pn, nd.sess.Lease())
		if err := nd.MkMountSymlink(pn, mnt, nd.sess.Lease()); err != nil {
			db.DPrintf(db.NAMED, "mount %v at %v err %v\n", nd.realm, pn, err)
			return err
		}

		// Signal realmd we are ready
		if err := sem.Up(); err != nil {
			db.DPrintf(db.NAMED, "%v sem up %v err %v\n", nd.realm, sem.String(), err)
			return err
		}
	}

	nd.getRoot(pn)

	if err := nd.CreateLeaderFile(path.Join(sp.NAME, nd.elect.Key()), nil, sp.TleaseId(nd.sess.Lease())); err != nil {
		db.DPrintf(db.NAMED, "CreateElectionInfo %v err %v\n", nd.elect.Key(), err)
	}

	if nd.crash > 0 {
		crash.Crasher(nd.SigmaClnt.FsLib)
	}

	<-ch

	db.DPrintf(db.NAMED, "%v: named done %v %v\n", proc.GetPid(), nd.realm, mnt)

	if err := nd.resign(); err != nil {
		db.DPrintf(db.NAMED, "resign %v err %v\n", proc.GetPid(), err)
	}

	nd.SigmaSrv.Exit(proc.MakeStatus(proc.StatusEvicted))

	return nil
}

func (nd *Named) mkSrv() error {
	ip, err := container.LocalIP()
	if err != nil {
		return err
	}

	root := rootDir(nd.fs, nd.realm)
	srv := fslibsrv.BootSrv(root, ip+":0", nd.SigmaClnt, nd.attach, nd.detach, nil, nil)
	if srv == nil {
		return fmt.Errorf("BootSrv err %v\n", err)
	}

	ssrv := sigmasrv.MakeSigmaSrvSess(srv, sp.Tuname(proc.GetPid().String()))
	if err := ssrv.MountRPCSrv(newLeaseSrv(nd.fs)); err != nil {
		return err
	}

	db.DPrintf(db.NAMED, "mkSrv %v %v %v\n", nd.realm, srv.MyAddr(), nd.elect.Key())

	nd.SigmaSrv = ssrv
	return nil
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
		err := nd.WaitEvict(proc.GetPid())
		if err != nil {
			db.DPrintf(db.NAMED, "Error WaitEvict: %v", err)
			time.Sleep(time.Second)
			continue
		}
		db.DPrintf(db.NAMED, "candidate %v %v evicted\n", nd.realm, proc.GetPid().String())
		ch <- struct{}{}
	}
}
