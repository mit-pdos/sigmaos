package named

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.etcd.io/etcd/client/v3/concurrency"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/etcdclnt"
	"sigmaos/fslibsrv"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func RunKNamed(args []string) error {
	db.DPrintf(db.NAMED, "%v: knamed %v\n", proc.GetPid(), args)
	if len(args) != 2 {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}
	nd = &Named{}
	ch := make(chan struct{})
	nd.realm = sp.Trealm(args[1])

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
		db.DFatalf("%v: LocalIP %v %v\n", proc.GetPid(), err)
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

	if err := ec.SetRootNamed(mnt, electclnt.Key(), electclnt.Rev()); err != nil {
		db.DFatalf("SetNamed: %v", err)
	}
	sc, err := sigmaclnt.MkSigmaClntFsLib(proc.GetPid().String())
	if err != nil {
		db.DFatalf("MkSigmaClntFsLib: err %v", err)
	}
	nd.SigmaClnt = sc

	// go nd.exit(ch)
	nd.initfs()
	w := os.NewFile(uintptr(3), "pipe")
	fmt.Fprintf(w, "started")
	w.Close()

	<-ch

	db.DPrintf(db.NAMED, "leader %v %v done\n", nd.realm, mnt)

	// XXX maybe clear boot block

	return nil
}

// for testing
func (nd *Named) exit(ch chan struct{}) {
	time.Sleep(2 * time.Second)
	db.DPrintf(db.NAMED, "boot named exit\n")
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
