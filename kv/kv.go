package kv

//
// A kev-value server implemented using memfs
//

import (
	"fmt"
	"log"
	"strings"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/inode"
	"ulambda/leaseclnt"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	GRP       = "grp-"
	GRPCONF   = "-conf"
	GRPCONFBK = "-confbk"
	CTL       = "ctl"
)

func grpconf(grp string) string {
	return KVDIR + "/" + grp + GRPCONF

}

func grpconfbk(grp string) string {
	return KVDIR + "/" + grp + GRPCONFBK

}

type Kv struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	crash     int64
	primLease *leaseclnt.LeaseClnt
	lease     *leaseclnt.LeaseClnt
	conf      *kvConf
}

func RunKv(grp string) {
	kvd := &Kv{}
	kvd.FsLib = fslib.MakeFsLib("kv-" + proc.GetPid())
	kvd.ProcClnt = procclnt.MakeProcClnt(kvd.FsLib)
	kvd.crash = crash.GetEnv()

	kvd.primLease = leaseclnt.MakeLeaseClnt(kvd.FsLib, np.MEMFS+"/"+grp, np.DMSYMLINK)
	kvd.lease = leaseclnt.MakeLeaseClnt(kvd.FsLib, grpconf(grp), 0)

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

	kvd.primLease.WaitWLease(fslib.MakeTarget(mfs.MyAddr()))

	log.Printf("%v: primary %v\n", db.GetName(), grp)

	select {
	case <-ch:
		// finally primary, but done
	default:
		// run until we are told to stop
		kvd.recover(grp)
		<-ch
	}

	log.Printf("%v: done %v\n", db.GetName(), grp)

	mfs.Done()
}

func (kvd *Kv) recover(grp string) {
	var err error
	kvd.conf, err = readKvConf(kvd.FsLib, grpconf(grp))
	if err == nil {
		log.Printf("%v: recovery: use %v\n", db.GetName(), kvd.conf)
		return
	}
	// roll back to old conf
	fn := grpconfbk(grp)
	err = kvd.lease.MakeLeaseFileFrom(fn)
	if err != nil {
		log.Printf("%v: MakeLeaseFileFrom %v err %v\n", db.GetName(), fn, err)
		// this must be the first recovery of the kv group;
		// otherwise, there would be a either a config or
		// backup config.
		err = kvd.lease.MakeLeaseFileJson(kvConf{"kv-" + proc.GetPid(), []string{}})
		if err != nil {
			log.Fatalf("%v: recover failed to create initial config\n", db.GetName())
		}
	} else {
		log.Printf("%v: recovery: restored config from %v\n", db.GetName(), fn)
	}
}

func (kvd *Kv) op(opcode, kv string) {
	log.Printf("%v: opcode %v kv %v\n", db.GetName(), opcode, kv)
}

type kvConf struct {
	primary string
	backups []string
}

func readKvConf(fsl *fslib.FsLib, conffile string) (*kvConf, error) {
	conf := kvConf{}
	err := fsl.ReadFileJson(conffile, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

func KvGroupOp(fsl *fslib.FsLib, primary, opcode, kv string) error {
	s := opcode + " " + kv
	err := fsl.WriteFile(primary+"/"+CTL, []byte(s))
	return err
}

type KvCtl struct {
	fs.FsObj
	kvd *Kv
}

func makeKvCtl(uname string, parent fs.Dir, kv *Kv) fs.FsObj {
	i := inode.MakeInode(uname, np.DMDEVICE, parent)
	return &KvCtl{i, kv}
}

func (c *KvCtl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, fmt.Errorf("Invalid arguments")
	}
	c.kvd.op(words[0], words[1])
	return np.Tsize(len(b)), nil
}

func (c *KvCtl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	return nil, nil
}
