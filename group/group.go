package group

//
// A group of servers with a primary and one or more backups
//

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"ulambda/atomic"
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
	GRPDIR       = "name/group"
	GRP          = "grp-"
	GRPCONF      = "-conf"
	GRPCONFNXT   = "-conf-next"
	GRPCONFNXTBK = GRPCONFNXT + "#"
	CTL          = "ctl"
)

func GrpConfPath(grp string) string {
	return GRPDIR + "/" + grp + GRPCONF

}

func grpConfNxt(grp string) string {
	return GRPDIR + "/" + grp + GRPCONFNXT
}

func grpConfNxtBk(grp string) string {
	return GRPDIR + "/" + grp + GRPCONFNXTBK

}

type Group struct {
	sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	crash        int64
	primFence    *fenceclnt.FenceClnt
	confFclnt    *fenceclnt.FenceClnt
	conf         *GrpConf
	isRecovering bool
}

func (g *Group) testAndSetRecovering() bool {
	g.Lock()
	defer g.Unlock()
	b := g.isRecovering
	if !g.isRecovering {
		g.isRecovering = true
	}
	return b
}

func (g *Group) setRecovering(b bool) {
	g.Lock()
	defer g.Unlock()
	g.isRecovering = b
}

func RunMember(grp string) {
	g := &Group{}
	g.FsLib = fslib.MakeFsLib("kv-" + proc.GetPid())
	g.ProcClnt = procclnt.MakeProcClnt(g.FsLib)
	g.crash = crash.GetEnv()

	g.Mkdir(GRPDIR, 07)

	g.primFence = fenceclnt.MakeFenceClnt(g.FsLib, GRPDIR+"/"+grp, np.DMSYMLINK)
	g.confFclnt = fenceclnt.MakeFenceClnt(g.FsLib, GrpConfPath(grp), 0)

	g.setRecovering(true)

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

func (g *Group) PublishConfig(grp string) {
	bk := grpConfNxtBk(grp)
	err := g.Remove(bk)
	if err != nil {
		log.Printf("%v: Remove %v err %v\n", db.GetName(), bk, err)
	}
	err = atomic.MakeFileJsonAtomic(g.FsLib, bk, 0777, *g.conf)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFile %v err %v\n", db.GetName(), bk, err)
	}
	err = g.confFclnt.OpenFenceFrom(bk)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFenceFileFrom err %v\n", db.GetName(), err)
	}
}

// nothing to restore yet
func (g *Group) restore(grp string) {
}

func (g *Group) recover(grp string) {
	var err error
	g.conf, err = readGroupConf(g.FsLib, grpConfNxt(grp))
	if err == nil {
		g.restore(grp)
	} else {
		// this must be the first recovery of the balancer;
		// otherwise, there would be a either a config or
		// backup config.
		g.conf = &GrpConf{"kv-" + proc.GetPid(), []string{}}
		g.PublishConfig(grp)
	}
}

func (g *Group) op(opcode, kv string) error {
	if g.testAndSetRecovering() {
		return fmt.Errorf("retry")
	}
	defer g.setRecovering(false)

	log.Printf("%v: opcode %v kv %v\n", db.GetName(), opcode, kv)
	return nil
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
	err := c.g.op(words[0], words[1])
	return np.Tsize(len(b)), err
}

func (c *GroupCtl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	return nil, nil
}
