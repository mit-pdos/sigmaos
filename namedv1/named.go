package namedv1

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

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
	bootNamed := len(args) == 2 // XXX args[1] is realm
	db.DPrintf(db.NAMEDV1, "%v: BootNamed %v %v\n", proc.GetPid(), bootNamed, args)
	if !(len(args) == 2 || len(args) == 3) {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}
	nd = &Named{}
	ch := make(chan struct{})
	if bootNamed {
		nd.realm = sp.Trealm(args[1])
	} else {
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
	}

	db.DPrintf(db.NAMEDV1, "started %v %v %v\n", proc.GetPid(), nd.realm, proc.GetRealm())

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
	db.DPrintf(db.NAMEDV1, "candidate %v %v\n", proc.GetPid().String(), fn)

	electclnt := concurrency.NewElection(nd.sess, fn)

	if err := electclnt.Campaign(context.TODO(), proc.GetPid().String()); err != nil {
		db.DFatalf("Campaign err %v\n", err)
	}

	resp, err := electclnt.Leader(context.TODO())
	if err != nil {
		db.DFatalf("Leader err %v\n", err)
	}

	db.DPrintf(db.NAMEDV1, "leader %v %v\n", proc.GetPid().String(), resp)
	root := rootDir(ec, nd.realm)
	srv := fslibsrv.BootSrv(root, ip+":0", "namedv1", nd.SigmaClnt)
	if srv == nil {
		db.DFatalf("MakeReplServer err %v", err)
	}
	nd.SessSrv = srv

	mnt := sp.MkMountServer(srv.MyAddr())

	db.DPrintf(db.NAMEDV1, "leader %v %v\n", nd.realm, mnt)

	if nd.realm == sp.ROOTREALM {
		if err := ec.SetRootNamed(mnt, electclnt.Key(), electclnt.Rev()); err != nil {
			db.DFatalf("SetNamed: %v", err)
		}
		sc, err := sigmaclnt.MkSigmaClntFsLib(proc.GetPid().String())
		if err != nil {
			db.DFatalf("MkSigmaClntFsLib: err %v", err)
		}
		nd.SigmaClnt = sc
	} else {
		// note: the named proc runs in rootrealm
		pn := path.Join(sp.REALMS, nd.realm.String())
		db.DPrintf(db.NAMEDV1, "mount %v at %v\n", nd.realm, pn)
		if err := nd.MkMountSymlink(pn, mnt); err != nil {
			db.DPrintf(db.NAMEDV1, "mount %v at %v err %v\n", nd.realm, pn, err)
			return err
		}
		sts, err := nd.GetDir(sp.REALMS)
		if err != nil {
			db.DPrintf(db.NAMEDV1, "getdir %v err %v\n", sp.REALMS, err)
			return err
		}
		db.DPrintf(db.NAMEDV1, "getdir %v sts %v\n", sp.REALMS, sp.Names(sts))
	}

	if bootNamed {
		// go nd.exit(ch)
		nd.initfs()
		w := os.NewFile(uintptr(3), "pipe")
		fmt.Fprintf(w, "started")
		w.Close()
	} else {
		nd.Started()
		go nd.waitExit(ch)
		if nd.crash > 0 {
			crash.Crasher(nd.SigmaClnt.FsLib)
		}
	}

	<-ch

	db.DPrintf(db.NAMEDV1, "leader %v done\n", mnt)

	// XXX maybe clear boot block

	if !bootNamed {
		nd.Exited(proc.MakeStatus(proc.StatusEvicted))
	}

	return nil
}

func (nd *Named) waitExit(ch chan struct{}) {
	err := nd.WaitEvict(proc.GetPid())
	if err != nil {
		db.DFatalf("Error WaitEvict: %v", err)
	}
	db.DPrintf(db.NAMEDV1, "candidate %v evicted\n", proc.GetPid().String())
	ch <- struct{}{}
}

// for testing
func (nd *Named) exit(ch chan struct{}) {
	time.Sleep(2 * time.Second)
	db.DPrintf(db.NAMEDV1, "boot named exit\n")
	ch <- struct{}{}
}

var InitRootDir = []string{sp.BOOT, sp.KPIDS, sp.SCHEDD, sp.UX, sp.S3, sp.DB}

// If initial root dir doesn't exist, create it.
func (nd *Named) initfs() error {
	// XXX clean up WS here for now
	if err := nd.RmDir(sp.WS); err != nil {
		db.DPrintf(db.ALWAYS, "Failed to clean up %v err %v", sp.WS, err)
	}
	for _, n := range InitRootDir {
		_, err := nd.Create(n, 0777|sp.DMDIR, sp.OREAD)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error create [%v]: %v", n, err)
			return err
		}
	}
	return nil
}
