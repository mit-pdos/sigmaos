package group

//
// A group of servers with a primary and one or more backups
//

import (
	"log"
	"strings"
	"sync"

	"ulambda/atomic"
	"ulambda/crash"
	"ulambda/fenceclnt"
	"ulambda/fidclnt"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/inode"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/replraft"
)

const (
	GRPDIR       = "name/group/"
	GRP          = "grp-"
	GRPPRIM      = "-primary"
	GRPPEER      = "-peer-fence"
	GRPRAFTCONF  = "-raft-conf"
	GRPCONF      = "-conf"
	GRPCONFNXT   = "-conf-next"
	GRPCONFNXTBK = GRPCONFNXT + "#"
	CTL          = "ctl"
)

func GrpDir(grp string) string {
	return GRPDIR + grp + "/"
}

func GrpConfPath(grp string) string {
	return GRPDIR + grp + GRPCONF
}

func grpConfNxt(grp string) string {
	return GRPDIR + grp + GRPCONFNXT
}

func grpConfNxtBk(grp string) string {
	return GRPDIR + grp + GRPCONFNXTBK
}

func grpRaftAddrs(grp string) string {
	return GRPDIR + grp + GRPRAFTCONF
}

func grpPrimFPath(grp string) string {
	return GRPDIR + grp + GRPPRIM
}

func grpPeerFPath(grp string) string {
	return GRPDIR + grp + GRPPEER
}

type ReplicaAddrs struct {
	SigmaAddrs   []string
	RaftsrvAddrs []string
}

type Group struct {
	sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	crash        int64
	primFence    *fenceclnt.FenceClnt
	peerFence    *fenceclnt.FenceClnt
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

	srvs := []string{GRPDIR}
	g.primFence = fenceclnt.MakeFenceClnt(g.FsLib, grpPrimFPath(grp), 0, srvs)
	g.peerFence = fenceclnt.MakeFenceClnt(g.FsLib, grpPeerFPath(grp), 0, srvs)
	g.confFclnt = fenceclnt.MakeFenceClnt(g.FsLib, GrpConfPath(grp), 0, srvs)

	g.setRecovering(true)

	// Under fence: get peers, start server and add self to list of peers, update list of peers.
	g.peerFence.AcquireFenceW([]byte{})
	replicaAddrs := g.readReplicaAddrs(grp)

	ip, err := fidclnt.LocalIP()
	if err != nil {
		log.Fatalf("FATAL group ip %v\n", err)
	}

	replicaAddrs.RaftsrvAddrs = append(replicaAddrs.RaftsrvAddrs, ip+":0")
	id := len(replicaAddrs.RaftsrvAddrs)

	raftConfig := replraft.MakeRaftConfig(id, replicaAddrs.RaftsrvAddrs)

	// start server but don't publish its existence
	mfs, err1 := fslibsrv.MakeReplMemFsFsl(replicaAddrs.RaftsrvAddrs[id-1], "", g.FsLib, g.ProcClnt, raftConfig)
	if err1 != nil {
		log.Fatalf("FATAL StartMemFs %v\n", err1)
	}

	// Get the final repl addr
	replicaAddrs.RaftsrvAddrs[id-1] = raftConfig.ReplAddr()
	replicaAddrs.SigmaAddrs = append(replicaAddrs.SigmaAddrs, mfs.MyAddr())

	g.writeReplicaAddrs(grp, replicaAddrs)

	// Add symlink
	atomic.PutFileAtomic(g.FsLib, GrpDir(grp), 0777|np.DMSYMLINK, fslib.MakeTarget(replicaAddrs.SigmaAddrs))
	g.peerFence.ReleaseFence()

	// start server and write ch when server is done
	ch := make(chan bool)
	go func() {
		mfs.Serve()
		ch <- true
	}()

	g.primFence.AcquireFenceW([]byte(mfs.MyAddr()))

	log.Printf("%v: primary %v\n", proc.GetProgram(), grp)

	select {
	case <-ch:
		// finally primary, but done
	default:
		// run until we are told to stop
		g.recover(grp)
		<-ch
	}

	log.Printf("%v: group done %v\n", proc.GetProgram(), grp)

	mfs.Done()
}

func (g *Group) readReplicaAddrs(grp string) *ReplicaAddrs {
	ra := &ReplicaAddrs{}
	err := g.GetFileJson(grpRaftAddrs(grp), ra)
	if err != nil {
		log.Printf("Error GetFileJson: %v", err)
	}
	return ra
}

func (g *Group) writeReplicaAddrs(grp string, ra *ReplicaAddrs) {
	err := atomic.PutFileJsonAtomic(g.FsLib, grpRaftAddrs(grp), 0777, ra)
	if err != nil {
		log.Fatalf("FATAL Error writeReplicaAddrs %v", err)
	}
}

func (g *Group) PublishConfig(grp string) {
	bk := grpConfNxtBk(grp)
	err := g.Remove(bk)
	if err != nil {
		log.Printf("%v: Remove %v err %v\n", proc.GetProgram(), bk, err)
	}
	err = atomic.PutFileJsonAtomic(g.FsLib, bk, 0777, *g.conf)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFile %v err %v\n", proc.GetProgram(), bk, err)
	}
	err = g.confFclnt.OpenFenceFrom(bk)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFenceFileFrom err %v\n", proc.GetProgram(), err)
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

func (g *Group) op(opcode, kv string) *np.Err {
	if g.testAndSetRecovering() {
		return np.MkErr(np.TErrRetry, "busy")
	}
	defer g.setRecovering(false)

	log.Printf("%v: opcode %v kv %v\n", proc.GetProgram(), opcode, kv)
	return nil
}

type GrpConf struct {
	primary string
	backups []string
}

func readGroupConf(fsl *fslib.FsLib, conffile string) (*GrpConf, error) {
	conf := GrpConf{}
	err := fsl.GetFileJson(conffile, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

func GroupOp(fsl *fslib.FsLib, primary, opcode, kv string) error {
	s := opcode + " " + kv
	_, err := fsl.SetFile(primary+"/"+CTL, []byte(s), 0)
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

func (c *GroupCtl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, np.MkErr(np.TErrInval, words)
	}
	err := c.g.op(words[0], words[1])
	return np.Tsize(len(b)), err
}

func (c *GroupCtl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	return nil, nil
}
