package kv

//
// A kev-value server implemented using memfs
//

import (
	"log"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/leaseclnt"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	GRP = "grp-"
)

type Kv struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	crash int64
	lease *leaseclnt.LeaseClnt
}

func RunKv(grp string) {
	kv := &Kv{}
	kv.FsLib = fslib.MakeFsLib("kv-" + proc.GetPid())
	kv.ProcClnt = procclnt.MakeProcClnt(kv.FsLib)
	kv.crash = crash.GetEnv()

	kv.lease = leaseclnt.MakeLeaseClnt(kv.FsLib, np.MEMFS+"/"+grp, np.DMSYMLINK)
	// kv.lease = leaseclnt.MakeLeaseClnt(kv.FsLib, KVCONFIG, 0)

	// start server but don't publish its existence
	mfs, _, err := fslibsrv.MakeMemFs("", "kv-"+proc.GetPid())
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}

	// start server and write ch when server is done
	ch := make(chan bool)
	go func() {
		mfs.Serve()
		ch <- true
	}()

	kv.lease.WaitWLease(fslib.MakeTarget(mfs.MyAddr()))

	log.Printf("%v: primary %v\n", db.GetName(), grp)

	select {
	case <-ch:
		// finally primary, but done
	default:
		// run until we are told to stop
		<-ch
	}

	log.Printf("%v: done %v\n", db.GetName(), grp)

	mfs.Done()

}
