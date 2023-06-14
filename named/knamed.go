package named

import (
	"fmt"
	"os"
	"time"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
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
	nd.realm = sp.Trealm(args[1])

	ec, err := etcdclnt.MkEtcdClnt(nd.realm)
	if err != nil {
		db.DFatalf("Error MkEtcdClnt %v\n", err)
	}
	nd.ec = ec

	db.DPrintf(db.NAMED, "started %v %v %v\n", proc.GetPid(), nd.realm, proc.GetRealm())

	if err := nd.startLeader(); err != nil {
		db.DFatalf("Error startLeader %v\n", err)
	}
	defer ec.Close()

	mnt := sp.MkMountServer(nd.MyAddr())
	if err := ec.SetRootNamed(mnt, nd.elect.Key(), nd.elect.Rev()); err != nil {
		db.DFatalf("SetNamed: %v", err)
	}
	sc, err := sigmaclnt.MkSigmaClntFsLib(proc.GetPid().String())
	if err != nil {
		db.DFatalf("MkSigmaClntFsLib: err %v", err)
	}
	nd.SigmaClnt = sc

	ch := make(chan struct{})
	// go nd.exit(ch)
	nd.initfs()
	w := os.NewFile(uintptr(3), "pipe")
	fmt.Fprintf(w, "started")
	w.Close()

	<-ch

	db.DPrintf(db.NAMED, "%v: leader %v %v done\n", proc.GetPid(), nd.realm, mnt)

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
