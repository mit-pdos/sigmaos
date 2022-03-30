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
	"ulambda/fidclnt"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/inode"
	"ulambda/leaderclnt"
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
	SigmaAddrs   []string
	RaftsrvAddrs []string
}

type Group struct {
	sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	lc     *leaderclnt.LeaderClnt
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

// Get the addresses of other replicas, set the addresses corresponding to id,
// and return the result, all while fenced. If id == -1, then this is a server
// trying to set up its addresses for the first time, so append to the address
// lists instead of setting.
func (g *Group) setAddrs(grp string, id int, sigmaAddr string, raftAddr string, fenceDirs []string) *ReplicaAddrs {
	var replicaAddrs *ReplicaAddrs
	// Retry if epochs expire
	for {
		if _, err := g.lc.EnterNextEpoch(fenceDirs); err != nil {
			// If we got a stale result, retry
			if np.IsErrStale(err) {
				log.Printf("Retry enter next epoch 1 %v err %v", grp, err)
				continue
			} else {
				log.Fatalf("FATAL couldn't enter next epoch 1 %v err %v", grp, err)
			}
		}
		if ra, err := g.readReplicaAddrs(grp); err != nil && !np.IsErrNotfound(err) {
			// If we got a stale result, retry
			if np.IsErrStale(err) {
				log.Printf("Retry read replica addrs %v err %v", grp, err)
				continue
			} else {
				log.Fatalf("FATAL couldn't read replica addrs %v err %v", grp, err)
			}
		} else {
			replicaAddrs = ra
		}
		// Update the stored addresses.
		if id < 0 {
			// If trying to set up a new server...
			replicaAddrs.SigmaAddrs = append(replicaAddrs.SigmaAddrs, sigmaAddr)
			replicaAddrs.RaftsrvAddrs = append(replicaAddrs.RaftsrvAddrs, raftAddr)
		} else {
			replicaAddrs.SigmaAddrs[id-1] = sigmaAddr
			replicaAddrs.RaftsrvAddrs[id-1] = raftAddr
		}
		if err := g.writeReplicaAddrs(grp, replicaAddrs); err != nil {
			// If we got a stale result, retry
			if np.IsErrStale(err) {
				log.Printf("Retry read replica addrs %v err %v", grp, err)
				continue
			} else {
				log.Fatalf("FATAL couldn't write replica addrs %v err %v", grp, err)
			}
		} else {
			// we're done
			break
		}
	}
	return replicaAddrs
}

func RunMember(grp string) {
	g := &Group{}
	g.isBusy = true
	g.FsLib = fslib.MakeFsLib("kv-" + proc.GetPid().String())
	g.ProcClnt = procclnt.MakeProcClnt(g.FsLib)
	g.lc = leaderclnt.MakeLeaderClnt(g.FsLib, GrpConfPath(grp), 0777)
	crash.Crasher(g.FsLib)

	fenceDirs := []string{GrpSym(grp), grpRaftAddrs(grp)}

	// XXX need this?
	g.MkDir(GRPDIR, 0777)

	replicated, err := strconv.ParseBool(os.Getenv("SIGMAREPL"))
	if err != nil {
		log.Fatalf("FATAL invalid sigmarepl: %v", err)
	}

	replicaAddrs := g.setAddrs(grp, -1, PLACEHOLDER_ADDR, PLACEHOLDER_ADDR, fenceDirs)

	ip, err := fidclnt.LocalIP()
	if err != nil {
		log.Fatalf("FATAL group ip %v\n", err)
	}

	id := len(replicaAddrs.RaftsrvAddrs)
	replicaAddrs.RaftsrvAddrs[id-1] = ip + ":0"

	var raftConfig *replraft.RaftConfig = nil
	if replicated {
		raftConfig = replraft.MakeRaftConfig(id, replicaAddrs.RaftsrvAddrs)
	}

	// start server but don't publish its existence
	mfs, err1 := fslibsrv.MakeReplMemFsFsl(replicaAddrs.RaftsrvAddrs[id-1], "", g.FsLib, g.ProcClnt, raftConfig)
	if err1 != nil {
		log.Fatalf("FATAL StartMemFs %v\n", err1)
	}

	// Get the final sigma and repl addr
	sigmaAddr := mfs.MyAddr()
	var raftAddr string

	if replicated {
		raftAddr = raftConfig.ReplAddr()
	}

	// Update the stored addresses in named.
	replicaAddrs = g.setAddrs(grp, id, sigmaAddr, raftAddr, fenceDirs)

	// Add symlink

	for {
		if _, err := g.lc.EnterNextEpoch(fenceDirs); err != nil {
			// If we got a stale result, retry
			if np.IsErrStale(err) {
				log.Printf("Retry enter next epoch 1 %v err %v", grp, err)
				continue
			} else {
				log.Fatalf("FATAL couldn't enter next epoch 1 %v err %v", grp, err)
			}
		}
		if err := atomic.PutFileAtomic(g.FsLib, GrpSym(grp), 0777|np.DMSYMLINK, fslib.MakeTarget(replicaAddrs.SigmaAddrs)); err != nil {
			// If we got a stale result, retry
			if np.IsErrStale(err) {
				log.Printf("Retry read replica addrs %v err %v", grp, err)
				continue
			} else {
				log.Fatalf("FATAL couldn't read replica addrs %v err %v", grp, err)
			}
		} else {
			// we're done
			break
		}
	}
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
