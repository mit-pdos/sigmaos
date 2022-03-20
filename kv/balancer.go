package kv

//
// A balancer, which acts as a coordinator for the sharded KV service.
// A KV service deployment has several balancers: one primary and
// several backups.
//
// When a client adds/removes a shard, the primary balancer updates
// KVCONF, which has the mapping from shards to groups in the
// following steps.
//
// If the balancer isn't the primary anymore (e.g., it is partitioned
// and another balancer has become primary), the old primary's writes
// will fail, because its fences have an old epoch.
//

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"ulambda/atomic"
	"ulambda/crash"
	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/group"
	"ulambda/inode"
	"ulambda/leaderclnt"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	NKV           = 10
	NSHARD        = 10 * NKV
	KVDIR         = "name/kv/"
	KVCONF        = "config"
	KVCONFIG      = KVDIR + KVCONF // file with current config
	KVBALANCER    = KVDIR + "balancer"
	KVBALANCERCTL = KVDIR + "balancer/ctl"
)

var FENCEDDIRS = []string{KVDIR}

type Balancer struct {
	sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	conf       *Config
	lc         *leaderclnt.LeaderClnt
	mo         *Monitor
	ch         chan bool
	crash      int64
	crashChild string
	isBusy     bool // in config change?
}

func (bl *Balancer) testAndSetIsBusy() bool {
	bl.Lock()
	defer bl.Unlock()
	b := bl.isBusy
	bl.isBusy = true
	return b
}

func (bl *Balancer) clearIsBusy() {
	bl.Lock()
	defer bl.Unlock()
	bl.isBusy = false
}

func shardPath(kvd, shard string) string {
	return group.GRPDIR + "/" + kvd + "/shard" + shard
}

func RunBalancer(crashChild string, auto string) {
	bl := &Balancer{}

	// reject requests for changes until after recovery
	bl.isBusy = true

	bl.FsLib = fslib.MakeFsLib("balancer-" + proc.GetPid())
	bl.ProcClnt = procclnt.MakeProcClnt(bl.FsLib)
	bl.crash = crash.GetEnv()
	bl.crashChild = crashChild

	// may fail if already exist
	bl.MkDir(KVDIR, 07)

	bl.lc = leaderclnt.MakeLeaderClnt(bl.FsLib, KVBALANCER, np.DMSYMLINK|077)

	// start server but don't publish its existence
	mfs, err := fslibsrv.MakeMemFsFsl("", bl.FsLib, bl.ProcClnt)
	if err != nil {
		log.Fatalf("FATAL StartMemFs %v\n", err)
	}
	ctx := ctx.MkCtx("balancer", 0, nil)
	err1 := dir.MkNod(ctx, mfs.Root(), "ctl", makeCtl(ctx, mfs.Root(), bl))
	if err1 != nil {
		log.Fatalf("FATAL MakeNod clone failed %v\n", err1)
	}

	// start server and write ch when server is done
	ch := make(chan bool)
	go func() {
		mfs.Serve()
		ch <- true
	}()

	epoch, err := bl.lc.AcquireFencedEpoch(fslib.MakeTarget([]string{mfs.MyAddr()}), FENCEDDIRS)
	if err != nil {
		log.Fatalf("FATAL %v: AcquireFenceEpoch %v\n", proc.GetName(), err)
	}

	db.DLPrintf(db.ALWAYS, "primary %v for epoch %v\n", proc.GetName(), epoch)

	select {
	case <-ch:
		// done
	default:
		bl.recover(epoch)

		bl.clearIsBusy()

		go bl.monitorMyself(ch)

		// first epoch is used to create a functional system
		// (e.g., creating shards), so don't crash then.
		if epoch > 1 {
			crash.Crasher(bl.FsLib)
		}

		if auto == "auto" {
			bl.mo = MakeMonitor(bl.FsLib, bl.ProcClnt)
			bl.ch = make(chan bool)
			go bl.monitor()
		}

		// run until we are told to stop
		<-ch
	}

	mfs.Done()

	if bl.mo != nil {
		bl.Done()
	}
}

func BalancerOp(fsl *fslib.FsLib, opcode, mfs string) error {
	s := opcode + " " + mfs
	_, err := fsl.SetFile(KVBALANCERCTL, []byte(s), np.OWRITE, 0)
	return err
}

type Ctl struct {
	fs.Inode
	bl *Balancer
}

func makeCtl(ctx fs.CtxI, parent fs.Dir, bl *Balancer) fs.Inode {
	i := inode.MakeInode(ctx, np.DMDEVICE, parent)
	return &Ctl{i, bl}
}

// XXX call balance() repeatedly for each server passed in to write
// XXX assumes one client that retries
func (c *Ctl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, np.MkErr(np.TErrInval, words)
	}
	err := c.bl.balance(words[0], words[1])
	if err != nil {
		return 0, err
	}
	return np.Tsize(len(b)), nil
}

func (c *Ctl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "Read")
}

func (bl *Balancer) monitor() {
	const MS = 1000
	for true {
		select {
		case <-bl.ch:
			return
		default:
			time.Sleep(time.Duration(MS) * time.Millisecond)
			bl.mo.doMonitor(bl.conf)
		}
	}
}

func (bl *Balancer) Done() {
	bl.ch <- true
}

// check if i am still primary; if not, terminate myself
func (bl *Balancer) monitorMyself(ch chan bool) {
	for true {
		time.Sleep(time.Duration(100) * time.Millisecond)
		_, err := readConfig(bl.FsLib, KVCONFIG)
		if err != nil {
			if np.IsErrUnreachable(err) {
				// we are disconnected
				// log.Printf("%v: monitorMyself err %v\n", proc.GetName(), err)
				ch <- true
			}
		}
	}
}

// Done with deleters and movers; update config before indicating to
// caller that we are done.
func (bl *Balancer) clearMoves() {
	bl.conf.Moves = Moves{}
	db.DLPrintf("KVBAL", "Update config %v\n", bl.conf)
	bl.PublishConfig()
}

// Publish config atomically
func (bl *Balancer) PublishConfig() {
	err := atomic.PutFileJsonAtomic(bl.FsLib, KVCONFIG, 0777, *bl.conf)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFile %v err %v\n", proc.GetName(), KVCONFIG, err)
	}
}

// Publish new epoch, and Restore sharddirs by finishing moves and
// deletes.
func (bl *Balancer) restore(conf *Config, epoch np.Tepoch) {
	bl.conf = conf
	// Increase epoch, even if the config is the same as before,
	// so that helpers and clerks realize there is new balancer.
	bl.conf.Epoch = epoch
	db.DLPrintf("KVBAL", "restore to %v with epoch %v\n", bl.conf, epoch)
	bl.PublishConfig()
	bl.runMovers(bl.conf.Moves)
	bl.runDeleters(bl.conf.Moves)
	bl.clearMoves()
}

func (bl *Balancer) recover(epoch np.Tepoch) {
	conf, err := readConfig(bl.FsLib, KVCONFIG)
	if err == nil {
		bl.restore(conf, epoch)
	} else {
		// this must be the first recovery of the balancer;
		// otherwise, there would be a either a config or
		// backup config.
		bl.conf = MakeConfig(epoch)
		bl.PublishConfig()
	}
}

// Make intial shard directories
func (bl *Balancer) initShards(nextShards []string) {
	for s, kvd := range nextShards {
		dst := shardPath(kvd, shard(s))
		// Mkdir may fail because balancer crashed during config 0
		// so ignore error
		if err := bl.MkDir(dst, 0777); err != nil {
			db.DLPrintf("KVBAL_ERR", "warning mkdir %v err %v\n", dst, err)
		}
	}
}

func (bl *Balancer) spawnProc(args []string) (string, error) {
	p := proc.MakeProc(args[0], args[1:])
	p.AppendEnv("SIGMACRASH", bl.crashChild)
	err := bl.Spawn(p)
	if err != nil {
		db.DLPrintf("KVBAL_ERR", "spawn pid %v err %v\n", p.Pid, err)
	}
	return p.Pid, err
}

func (bl *Balancer) runProc(args []string) (*proc.Status, error) {
	pid, err := bl.spawnProc(args)
	if err != nil {
		return nil, err
	}
	status, err := bl.WaitExit(pid)
	return status, err
}

func (bl *Balancer) runProcRetry(args []string, retryf func(error, *proc.Status) bool) (error, *proc.Status) {
	var status *proc.Status
	var err error
	for true {
		status, err = bl.runProc(args)
		if err != nil {
			db.DLPrintf("ALWAYS", "runProc %v err %v status %v\n", args, err, status)
		}
		if err != nil && (strings.HasPrefix(err.Error(), "Spawn error") ||
			strings.HasPrefix(err.Error(), "Missing return status") ||
			np.IsErrUnreachable(err)) {
			log.Fatalf("CRASH %v: runProc err %v\n", proc.GetName(), err)
		}
		if retryf(err, status) {
			db.DLPrintf("KVBAL_ERR", "retry %v err %v status %v\n", args, err, status)
		} else {
			break
		}
	}
	return err, status
}

func (mvs Moves) moved() []string {
	srcs := []string{}
	for _, m := range mvs {
		srcs = append(srcs, m.Src)
	}
	return srcs
}

func (bl *Balancer) computeMoves(nextShards []string) Moves {
	moves := Moves{}
	for i, kvd := range bl.conf.Shards {
		if kvd != nextShards[i] {
			shard := shard(i)
			s := shardPath(kvd, shard)
			d := shardPath(nextShards[i], shard)
			moves = append(moves, &Move{s, d})
		}
	}
	return moves
}

func (bl *Balancer) checkMoves(moves Moves) {
	for _, m := range moves {
		fn := m.Dst
		_, err := bl.Stat(fn)
		if err != nil {
			log.Printf("%v: stat %v err %v\n", proc.GetName(), fn, err)
		}
	}
}

// Run deleters in parallel
func (bl *Balancer) runDeleters(moves Moves) {
	tmp := make(Moves, len(moves))
	ch := make(chan int)
	for i, m := range moves {
		go func(m *Move, i int) {
			bl.runProcRetry([]string{"bin/user/kv-deleter", bl.conf.Epoch.String(), m.Src}, func(err error, status *proc.Status) bool {
				retry := err != nil || (!status.IsStatusOK() && !np.IsErrNotfound(status))
				db.DLPrintf("KVBAL", "delete %v/%v done err %v status %v\n", i, m.Src, err, status)
				return retry
			})
			ch <- i
		}(m, i)
	}
	m := 0
	for range moves {
		i := <-ch
		tmp[i] = nil
		m += 1
		db.DLPrintf("KVBAL", "deleter done %v %v\n", m, tmp)
	}
	db.DLPrintf("KVBAL", "deleters done\n")
}

// Run movers in parallel
func (bl *Balancer) runMovers(moves Moves) {
	tmp := make(Moves, len(moves))
	copy(tmp, moves)
	ch := make(chan int)
	for i, m := range moves {
		go func(m *Move, i int) {
			bl.runProcRetry([]string{"bin/user/kv-mover", bl.conf.Epoch.String(), m.Src, m.Dst}, func(err error, status *proc.Status) bool {
				db.DLPrintf("KVBAL", "%v: move %v m %v err %v status %v\n", bl.conf.Epoch, i, m, err, status)
				return err != nil || !status.IsStatusOK()
			})
			ch <- i
		}(m, i)
	}
	m := 0
	for range moves {
		i := <-ch
		tmp[i] = nil
		m += 1
		db.DLPrintf("KVBAL", "mover done %v %v\n", m, tmp)
	}
	db.DLPrintf("KVBAL", "%v: movers all done\n", bl.conf.Epoch)
}

func (bl *Balancer) balance(opcode, mfs string) *np.Err {
	if bl.testAndSetIsBusy() {
		return np.MkErr(np.TErrRetry, fmt.Sprintf("busy %v", proc.GetName()))
	}
	defer bl.clearIsBusy()

	var nextShards []string
	switch opcode {
	case "add":
		if bl.conf.Present(mfs) {
			return nil
		}
		nextShards = AddKv(bl.conf, mfs)
	case "del":
		if !bl.conf.Present(mfs) {
			return nil
		}
		nextShards = DelKv(bl.conf, mfs)
	default:
	}

	var moves Moves
	docrash := false
	if bl.conf.Epoch == 1 {
		bl.initShards(nextShards)
		docrash = true
	} else {
		moves = bl.computeMoves(nextShards)
	}

	epoch, err := bl.lc.EnterNextEpoch(FENCEDDIRS)
	if err != nil {
		db.DLPrintf("KVBAL_ERR", "EnterNextEpoch fail %v\n", err)
		var nperr *np.Err
		if errors.As(err, &nperr) {
			return nperr
		}
		return np.MkErr(np.TErrError, err)
	}

	bl.conf.Epoch = epoch
	bl.conf.Shards = nextShards
	bl.conf.Moves = moves

	db.DLPrintf("KVBAL", "New config %v\n", bl.conf)

	// If balancer crashes, before here, KVCONFIG has the old
	// config; otherwise, the new conf.
	bl.PublishConfig()

	bl.runMovers(moves)

	bl.checkMoves(moves)

	bl.runDeleters(moves)

	bl.clearMoves()

	if docrash { // start crashing?
		crash.Crasher(bl.FsLib)
	}

	return nil
}
