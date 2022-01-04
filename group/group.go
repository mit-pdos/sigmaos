package group

//
// A group of servers with a primary and one or more backups
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
	GRPDIR    = "name/group"
	GRP       = "grp-"
	GRPCONF   = "-conf"
	GRPCONFBK = "-confbk"
	CTL       = "ctl"
)

func GrpConfPath(grp string) string {
	return GRPDIR + "/" + grp + GRPCONF

}

func grpconfbk(grp string) string {
	return GRPDIR + "/" + grp + GRPCONFBK

}

type Group struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	crash     int64
	primLease *leaseclnt.LeaseClnt
	lease     *leaseclnt.LeaseClnt
	conf      *GrpConf
}

func RunMember(grp string) {
	mg := &Group{}
	mg.FsLib = fslib.MakeFsLib("kv-" + proc.GetPid())
	mg.ProcClnt = procclnt.MakeProcClnt(mg.FsLib)
	mg.crash = crash.GetEnv()

	mg.Mkdir(GRPDIR, 07)

	mg.primLease = leaseclnt.MakeLeaseClnt(mg.FsLib, GRPDIR+"/"+grp, np.DMSYMLINK)
	mg.lease = leaseclnt.MakeLeaseClnt(mg.FsLib, GrpConfPath(grp), 0)

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

	mg.primLease.WaitWLease(fslib.MakeTarget(mfs.MyAddr()))

	log.Printf("%v: primary %v\n", db.GetName(), grp)

	select {
	case <-ch:
		// finally primary, but done
	default:
		// run until we are told to stop
		mg.recover(grp)
		<-ch
	}

	log.Printf("%v: done %v\n", db.GetName(), grp)

	mfs.Done()
}

func (mg *Group) recover(grp string) {
	var err error
	mg.conf, err = readGroupConf(mg.FsLib, GrpConfPath(grp))
	if err == nil {
		log.Printf("%v: recovery: use %v\n", db.GetName(), mg.conf)
		return
	}
	// roll back to old conf
	fn := grpconfbk(grp)
	err = mg.lease.MakeLeaseFileFrom(fn)
	if err != nil {
		log.Printf("%v: MakeLeaseFileFrom %v err %v\n", db.GetName(), fn, err)
		// this must be the first recovery of the kv group;
		// otherwise, there would be a either a config or
		// backup config.
		err = mg.lease.MakeLeaseFileJson(GrpConf{"kv-" + proc.GetPid(), []string{}})
		if err != nil {
			log.Fatalf("%v: recover failed to create initial config\n", db.GetName())
		}
	} else {
		log.Printf("%v: recovery: restored config from %v\n", db.GetName(), fn)
	}
}

func (mg *Group) op(opcode, kv string) {
	log.Printf("%v: opcode %v kv %v\n", db.GetName(), opcode, kv)
}

type GrpConf struct {
	primary string
	backups []string
}

func readGroupConf(fsl *fslib.FsLib, conffile string) (*GrpConf, error) {
	conf := GrpConf{}
	err := fsl.ReadFileJson(conffile, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

func GroupOp(fsl *fslib.FsLib, primary, opcode, kv string) error {
	s := opcode + " " + kv
	err := fsl.WriteFile(primary+"/"+CTL, []byte(s))
	return err
}

type GroupCtl struct {
	fs.FsObj
	mg *Group
}

func makeGroupCtl(uname string, parent fs.Dir, kv *Group) fs.FsObj {
	i := inode.MakeInode(uname, np.DMDEVICE, parent)
	return &GroupCtl{i, kv}
}

func (c *GroupCtl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, fmt.Errorf("Invalid arguments")
	}
	c.mg.op(words[0], words[1])
	return np.Tsize(len(b)), nil
}

func (c *GroupCtl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	return nil, nil
}
