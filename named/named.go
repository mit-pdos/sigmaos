package named

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"sync"

	"go.etcd.io/etcd/client/v3/concurrency"

	"sigmaos/container"
	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/etcdclnt"
	"sigmaos/fslibsrv"
	"sigmaos/proc"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

var nd *Named

type Named struct {
	*sigmaclnt.SigmaClnt
	*sesssrv.SessSrv
	mu    sync.Mutex
	ec    *etcdclnt.EtcdClnt
	sess  *concurrency.Session
	job   string
	realm sp.Trealm
	crash int
}

func Run(args []string) error {
	db.DPrintf(db.NAMED, "%v: %v\n", proc.GetPid(), args)
	if len(args) != 3 {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}
	nd = &Named{}
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
	// XXX nd.Started too soon for first named
	// nd.Started()
	// go nd.waitExit(ch)

	db.DPrintf(db.NAMED, "started %v %v %v\n", proc.GetPid(), nd.realm, proc.GetRealm())

	ec, err := etcdclnt.MkEtcdClnt(nd.realm)
	if err != nil {
		db.DFatalf("Error MkEtcdClnt %v\n", err)
	}
	nd.ec = ec

	s, err := concurrency.NewSession(ec.Client, concurrency.WithTTL(etcdclnt.SessionTTL))
	if err != nil {
		db.DFatalf("Error sess %v\n", err)
	}
	defer ec.Close()

	nd.sess = s

	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", sp.UX, err)
	}

	fn := fmt.Sprintf("named-election-%s", nd.realm)
	db.DPrintf(db.NAMED, "candidate %v %v\n", proc.GetPid().String(), fn)

	electclnt := concurrency.NewElection(nd.sess, fn)

	if err := electclnt.Campaign(context.TODO(), proc.GetPid().String()); err != nil {
		db.DFatalf("Campaign err %v\n", err)
	}

	resp, err := electclnt.Leader(context.TODO())
	if err != nil {
		db.DFatalf("Leader err %v\n", err)
	}

	db.DPrintf(db.NAMED, "leader %v %v\n", proc.GetPid().String(), resp)
	root := rootDir(ec, nd.realm)
	srv := fslibsrv.BootSrv(root, ip+":0", "named", nd.SigmaClnt)
	if srv == nil {
		db.DFatalf("MakeReplServer err %v", err)
	}
	nd.SessSrv = srv

	mnt := sp.MkMountServer(srv.MyAddr())

	db.DPrintf(db.NAMED, "leader %v %v\n", nd.realm, mnt)

	// note: the named proc runs in rootrealm
	pn := path.Join(sp.REALMS, nd.realm.String())
	db.DPrintf(db.NAMED, "mount %v at %v\n", nd.realm, pn)
	if err := nd.MkMountSymlink(pn, mnt); err != nil {
		db.DPrintf(db.NAMED, "mount %v at %v err %v\n", nd.realm, pn, err)
		return err
	}
	sts, err := nd.GetDir(sp.REALMS)
	if err != nil {
		db.DPrintf(db.NAMED, "getdir %v err %v\n", sp.REALMS, err)
		return err
	}
	db.DPrintf(db.NAMED, "getdir %v sts %v\n", sp.REALMS, sp.Names(sts))

	nd.Started()
	go nd.waitExit(ch)
	if nd.crash > 0 {
		crash.Crasher(nd.SigmaClnt.FsLib)
	}

	<-ch

	db.DPrintf(db.NAMED, "leader %v %v done\n", nd.realm, mnt)

	nd.Exited(proc.MakeStatus(proc.StatusEvicted))

	return nil
}

func (nd *Named) waitExit(ch chan struct{}) {
	err := nd.WaitEvict(proc.GetPid())
	if err != nil {
		db.DFatalf("Error WaitEvict: %v", err)
	}
	db.DPrintf(db.NAMED, "candidate %v %v evicted\n", nd.realm, proc.GetPid().String())
	ch <- struct{}{}
}
