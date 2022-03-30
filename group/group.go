package group

//
// A group of servers with a primary and one or more backups
//

import (
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"ulambda/atomic"
	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/electclnt"
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
	GRPDIR           = "name/group/"
	GRP              = "grp-"
	GRPRAFTCONF      = "-raft-conf"
	GRPCONF          = "-conf"
	GRPCONFNXT       = "-conf-next"
	GRPCONFNXTBK     = GRPCONFNXT + "#"
	CTL              = "ctl"
	PLACEHOLDER_ADDR = "PLACEHOLDER"
)

func GrpDir(grp string) string {
	return GRPDIR + grp + "/"
}

func GrpSym(grp string) string {
	return GRPDIR + grp
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

type ReplicaAddrs struct {
	SigmaAddrs []string
	RaftAddrs  []string
}

type Group struct {
	sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	ec     *electclnt.ElectClnt // We use an electclnt instead of an epochclnt because the config is stored in named. If we lose our connection to named & our leadership, we won't be able to write the config file anyway.
	conf   *GrpConf
	isBusy bool
}

func (g *Group) testAndSetBusy() bool {
	g.Lock()
	defer g.Unlock()
	b := g.isBusy
	g.isBusy = true
	return b
}

func (g *Group) clearBusy() {
	g.Lock()
	defer g.Unlock()
	g.isBusy = false
}

func RunMember(grp string) {
	g := &Group{}
	g.isBusy = true
	g.FsLib = fslib.MakeFsLib("kv-" + proc.GetPid().String())
	g.ProcClnt = procclnt.MakeProcClnt(g.FsLib)
	g.ec = electclnt.MakeElectClnt(g.FsLib, GrpConfPath(grp), 0777)
	crash.Crasher(g.FsLib)

	// XXX need this?
	g.MkDir(GRPDIR, 0777)

	replicated, err := strconv.ParseBool(os.Getenv("SIGMAREPL"))
	if err != nil {
		log.Fatalf("FATAL invalid sigmarepl: %v", err)
	}

	if err := g.ec.AcquireLeadership(nil); err != nil {
		log.Fatalf("FATAL AcquireLeadership in group.RunMember: %v", err)
	}

	// Read addrs
	replicaAddrs, err := g.readReplicaAddrs(grp)
	if err != nil && !np.IsErrNotfound(err) {
		log.Fatalf("FATAL readReplicaAddrs in group.RunMember: %v", err)
	}

	// Add placeholders to the addrs, and write them back to make sure the same
	// raft ID isn't reused.
	replicaAddrs.SigmaAddrs = append(replicaAddrs.SigmaAddrs, PLACEHOLDER_ADDR)
	replicaAddrs.RaftAddrs = append(replicaAddrs.RaftAddrs, PLACEHOLDER_ADDR)
	g.writeReplicaAddrs(grp, replicaAddrs)

	ip, err := fidclnt.LocalIP()
	if err != nil {
		log.Fatalf("FATAL group ip %v\n", err)
	}

	// Get raft id.
	id := len(replicaAddrs.SigmaAddrs)
	replicaAddrs.RaftAddrs[id-1] = ip + ":0"

	var raftConfig *replraft.RaftConfig = nil
	if replicated {
		raftConfig = replraft.MakeRaftConfig(id, replicaAddrs.RaftAddrs)
	}

	// start server but don't publish its existence
	mfs, err1 := fslibsrv.MakeReplMemFsFsl(replicaAddrs.RaftAddrs[id-1], "", g.FsLib, g.ProcClnt, raftConfig)
	if err1 != nil {
		log.Fatalf("FATAL StartMemFs %v\n", err1)
	}

	// Get the final sigma and repl addrs
	replicaAddrs.SigmaAddrs[id-1] = mfs.MyAddr()
	if replicated {
		replicaAddrs.RaftAddrs[id-1] = raftConfig.ReplAddr()
	}

	// Update the stored addresses in named.
	if err := g.writeReplicaAddrs(grp, replicaAddrs); err != nil {
		log.Fatalf("FATAL write replica addrs: %v", err)
	}

	// Clean sigma addrs, removing placeholders...
	sigmaAddrs := []string{}
	for _, a := range replicaAddrs.SigmaAddrs {
		if a != PLACEHOLDER_ADDR {
			sigmaAddrs = append(sigmaAddrs, a)
		}
	}

	if err := atomic.PutFileAtomic(g.FsLib, GrpSym(grp), 0777|np.DMSYMLINK, fslib.MakeTarget(sigmaAddrs)); err != nil {
		log.Fatalf("FATAL couldn't read replica addrs %v err %v", grp, err)
	}

	// Release leadership.
	if err := g.ec.ReleaseLeadership(); err != nil {
		log.Fatalf("FATAL release leadership: %v", err)
	}

	// XXX probably want to start these earlier...
	crash.Partitioner(mfs)
	crash.NetFailer(mfs)

	mfs.Serve()
	mfs.Done()
}

func (g *Group) readReplicaAddrs(grp string) (*ReplicaAddrs, error) {
	ra := &ReplicaAddrs{}
	err := g.GetFileJson(grpRaftAddrs(grp), ra)
	if err != nil {
		db.DLPrintf("GRP_ERR", "Error GetFileJson: %v", err)
		return ra, err
	}
	return ra, nil
}

func (g *Group) writeReplicaAddrs(grp string, ra *ReplicaAddrs) error {
	err := atomic.PutFileJsonAtomic(g.FsLib, grpRaftAddrs(grp), 0777, ra)
	if err != nil {
		return err
	}
	return nil
}

func (g *Group) op(opcode, kv string) *np.Err {
	if g.testAndSetBusy() {
		return np.MkErr(np.TErrRetry, "busy")
	}
	defer g.clearBusy()

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
	_, err := fsl.SetFile(primary+"/"+CTL, []byte(s), np.OWRITE, 0)
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
