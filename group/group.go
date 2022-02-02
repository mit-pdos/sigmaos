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
	"ulambda/fenceclnt"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/inode"
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
	primFence *fenceclnt.FenceClnt
	confFence *fenceclnt.FenceClnt
	conf      *GrpConf
}

func RunMember(grp string) {
	g := &Group{}
	g.FsLib = fslib.MakeFsLib("kv-" + proc.GetPid())
	g.ProcClnt = procclnt.MakeProcClnt(g.FsLib)
	g.crash = crash.GetEnv()

	g.Mkdir(GRPDIR, 07)

	g.primFence = fenceclnt.MakeFenceClnt(g.FsLib, GRPDIR+"/"+grp, np.DMSYMLINK)
	g.confFence = fenceclnt.MakeFenceClnt(g.FsLib, GrpConfPath(grp), 0)

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

	g.primFence.AcquireFenceW(fslib.MakeTarget(mfs.MyAddr()))

	log.Printf("%v: primary %v\n", db.GetName(), grp)

	select {
	case <-ch:
		// finally primary, but done
	default:
		// run until we are told to stop
		g.recover(grp)
		<-ch
	}

	log.Printf("%v: group done %v\n", db.GetName(), grp)

	mfs.Done()
}

func (g *Group) recover(grp string) {
	var err error
	g.conf, err = readGroupConf(g.FsLib, GrpConfPath(grp))
	if err == nil {
		log.Printf("%v: recovery: use %v\n", db.GetName(), g.conf)
		return
	}
	// no conf, roll back to old conf
	fn := grpconfbk(grp)
	err = g.ReadFileJson(fn, g.conf)
	if err != nil {
		log.Printf("%v: MakeFenceFileFrom %v err %v\n", db.GetName(), fn, err)
		// this must be the first recovery of the kv group;
		// otherwise, there would be a either a config or
		// backup config.
		err = g.MakeFileJson(GrpConfPath(grp), 0777|np.DMTMP, GrpConf{"kv-" + proc.GetPid(), []string{}})
		if err != nil {
			log.Fatalf("%v: recover failed to create initial config\n", db.GetName())
		}
	} else {
		log.Printf("%v: recovery: restored config %v from %v\n", db.GetName(), g.conf, fn)
	}
}

func (g *Group) op(opcode, kv string) {
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
	g *Group
}

func makeGroupCtl(ctx fs.CtxI, parent fs.Dir, kv *Group) fs.FsObj {
	i := inode.MakeInode(ctx, np.DMDEVICE, parent)
	return &GroupCtl{i, kv}
}

func (c *GroupCtl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, fmt.Errorf("Invalid arguments")
	}
	c.g.op(words[0], words[1])
	return np.Tsize(len(b)), nil
}

func (c *GroupCtl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	return nil, nil
}
