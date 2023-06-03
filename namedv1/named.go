package namedv1

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"sigmaos/container"
	"sigmaos/crash"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/etcdclnt"
	"sigmaos/fslibsrv"
	"sigmaos/proc"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

var (
	endpoints = []string{"127.0.0.1:2379", "localhost:22379", "localhost:32379"}
)

var nd *Named

type Named struct {
	*sigmaclnt.SigmaClnt
	*sesssrv.SessSrv
	mu    sync.Mutex
	clnt  *clientv3.Client
	sess  *concurrency.Session
	job   string
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
	if !bootNamed {
		nd.job = args[1]
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
	}

	db.DPrintf(db.NAMEDV1, "started %v\n", proc.GetPid())

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: etcdclnt.DialTimeout,
	})
	if err != nil {
		db.DFatalf("Error clientv3 %v\n", err)
	}
	nd.clnt = cli
	s, err := concurrency.NewSession(cli, concurrency.WithTTL(etcdclnt.SessionTTL))
	if err != nil {
		db.DFatalf("Error sess %v\n", err)
	}
	defer cli.Close()

	nd.sess = s

	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", sp.UX, err)
	}

	fn := "named-election"
	// fn := fmt.Sprintf("job-%s-election", nd.job))
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
	root := rootDir(cli)
	srv := fslibsrv.BootSrv(root, ip+":0", "namedv1", nd.SigmaClnt)
	if srv == nil {
		db.DFatalf("MakeReplServer err %v", err)
	}
	nd.SessSrv = srv

	mnt := sp.MkMountServer(srv.MyAddr())

	db.DPrintf(db.NAMEDV1, "leader %v\n", mnt)

	if err := etcdclnt.SetNamed(cli, mnt, electclnt.Key(), electclnt.Rev()); err != nil {
		db.DFatalf("SetNamed: %v", err)
	}

	if bootNamed {
		// go nd.exit(ch)
		initfs(root, InitRootDir)
		w := os.NewFile(uintptr(3), "pipe")
		fmt.Fprintf(w, "started")
		w.Close()
	} else if nd.crash > 0 {
		crash.Crasher(nd.SigmaClnt.FsLib)
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

// XXX only kernel dirs?
var InitRootDir = []string{sp.BOOTREL, sp.KPIDSREL, sp.SCHEDDREL, sp.UXREL, sp.S3REL, sp.DBREL}

func initfs(root *Dir, rootDir []string) error {
	for _, n := range rootDir {
		_, err := root.Create(ctx.MkCtx("", 0, nil), n, 0777|sp.DMDIR, sp.OREAD)
		if err != nil {
			db.DPrintf("Error create [%v]: %v", n, err)
			return err
		}
	}
	return nil
}