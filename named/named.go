package named

import (
	"fmt"
	"path"
	"strconv"
	"sync"
	"time"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/leaderetcd"
	"sigmaos/proc"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type Named struct {
	*sigmaclnt.SigmaClnt
	*sesssrv.SessSrv
	mu    sync.Mutex
	ec    *fsetcd.EtcdClnt
	elect *leaderetcd.Election
	job   string
	realm sp.Trealm
	crash int
}

func Run(args []string) error {
	db.DPrintf(db.NAMED, "%v: %v\n", proc.GetPid(), args)
	if len(args) != 3 {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}
	nd := &Named{}
	ch := make(chan struct{})
	nd.realm = sp.Trealm(args[1])
	crashing, err := strconv.Atoi(args[2])
	if err != nil {
		return fmt.Errorf("%v: crash %v isn't int", args[0], args[2])
	}
	nd.crash = crashing

	sc, err := sigmaclnt.MkSigmaClnt(proc.GetPid().String())
	if err != nil {
		return err
	}
	nd.SigmaClnt = sc

	nd.Started()
	go nd.waitExit(ch)

	db.DPrintf(db.NAMED, "started %v %v %v\n", proc.GetPid(), nd.realm, proc.GetRealm())

	if err := nd.startLeader(); err != nil {
		db.DPrintf(db.NAMED, "%v: startLeader %v err %v\n", proc.GetPid(), nd.realm, err)
		return err
	}
	defer nd.ec.Close()

	mnt := sp.MkMountServer(nd.MyAddr())

	pn := sp.NAMED
	if nd.realm == sp.ROOTREALM {
		db.DPrintf(db.ALWAYS, "SetRootNamed %v mnt %v\n", nd.realm, mnt)
		if err := nd.ec.SetRootNamed(mnt); err != nil {
			db.DFatalf("SetNamed: %v", err)
		}
	} else {
		// note: the named proc runs in rootrealm; maybe change it XXX
		pn = path.Join(sp.REALMS, nd.realm.String())
		db.DPrintf(db.ALWAYS, "mount %v at %v\n", nd.realm, pn)
		if err := nd.MkMountSymlink(pn, mnt); err != nil {
			db.DPrintf(db.NAMED, "mount %v at %v err %v\n", nd.realm, pn, err)
			return err
		}
	}

	nd.getRoot(pn)

	if nd.crash > 0 {
		crash.Crasher(nd.SigmaClnt.FsLib)
	}

	<-ch

	db.DPrintf(db.NAMED, "%v: named done %v %v\n", proc.GetPid(), nd.realm, mnt)

	if err := nd.resign(); err != nil {
		db.DPrintf(db.NAMED, "resign %v err %v\n", proc.GetPid(), err)
	}

	nd.Exited(proc.MakeStatus(proc.StatusEvicted))

	return nil
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
