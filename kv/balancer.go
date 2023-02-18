package kv

//
// A balancer, which acts as a coordinator for a sharded KV service.
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
	"strconv"
	"strings"
	"sync"
	"time"

	"sigmaos/crash"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/inode"
	"sigmaos/leaderclnt"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type Balancer struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	conf       *Config
	lc         *leaderclnt.LeaderClnt
	mo         *Monitor
	job        string
	kvdncore   proc.Tcore
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

func RunBalancer(job, crashChild, kvdncore string, auto string) {
	bl := &Balancer{}

	// reject requests for changes until after recovery
	bl.isBusy = true

	sc, err := sigmaclnt.MkSigmaClnt("balancer-" + proc.GetPid().String())
	bl.SigmaClnt = sc
	bl.job = job
	bl.crash = crash.GetEnv(proc.SIGMACRASH)
	bl.crashChild = crashChild
	var kvdnc int
	var error error
	kvdnc, error = strconv.Atoi(kvdncore)
	if error != nil {
		db.DFatalf("Bad kvdncore: %v", error)
	}
	bl.kvdncore = proc.Tcore(kvdnc)

	// may fail if already exist
	bl.MkDir(KVDIR, 07)
	bl.MkDir(JobDir(bl.job), 0777)

	bl.lc = leaderclnt.MakeLeaderClnt(bl.FsLib, KVBalancer(bl.job), sp.DMSYMLINK|077)

	// start server but don't publish its existence
	mfs, err := memfssrv.MakeMemFsPortClnt("", ":0", bl.SigmaClnt)
	if err != nil {
		db.DFatalf("StartMemFs %v\n", err)
	}
	ctx := ctx.MkCtx("balancer", 0, nil)
	err1 := dir.MkNod(ctx, mfs.Root(), "ctl", makeCtl(ctx, mfs.Root(), bl))
	if err1 != nil {
		db.DFatalf("MakeNod clone failed %v\n", err1)
	}

	// start server and write xch when server is done
	ch := make(chan bool)
	go func() {
		mfs.Serve()
		ch <- true
	}()

	mnt := sp.MkMountServer(mfs.MyAddr())
	b, error := mnt.Marshal()
	if error != nil {
		db.DFatalf("Marshal failed %v\n", error)
	}

	epoch, err := bl.lc.AcquireFencedEpoch(b, []string{})
	if err != nil {
		db.DFatalf("%v: AcquireFenceEpoch %v\n", proc.GetName(), err)
	}

	db.DPrintf(db.ALWAYS, "primary %v for epoch %v\n", proc.GetName(), epoch)

	// first epoch is used to create a functional system
	// (e.g., creating shards), so don't crash then.
	if epoch > 1 {
		crash.Crasher(bl.FsLib)
	}

	go bl.monitorMyself()

	select {
	case <-ch:
		// done
	default:
		bl.recover(epoch)

		bl.clearIsBusy()

		if auto == "auto" {
			bl.mo = MakeMonitor(bl.FsLib, bl.ProcClnt, bl.job, bl.kvdncore)
			bl.ch = make(chan bool)
			go bl.monitor()
		}

		// run until we are told to stop
		<-ch
	}

	db.DPrintf(db.KVBAL, "terminate\n")

	if bl.mo != nil {
		bl.ch <- true
		<-bl.ch
	}

	mfs.Done()
}

func BalancerOp(fsl *fslib.FsLib, job string, opcode, mfs string) error {
	s := opcode + " " + mfs
	_, err := fsl.SetFile(KVBalancerCtl(job), []byte(s), sp.OWRITE, 0)
	return err
}

// Retry a balancer op until success, or an unexpected error is returned.
func BalancerOpRetry(fsl *fslib.FsLib, job, opcode, mfs string) error {
	for true {
		err := BalancerOp(fsl, job, opcode, mfs)
		if err == nil {
			return nil
		}
		if serr.IsErrUnavailable(err) || serr.IsErrRetry(err) {
			// db.DPrintf(db.ALWAYS, "balancer op wait err %v\n", err)
			time.Sleep(100 * time.Millisecond)
		} else {
			db.DPrintf(db.ALWAYS, "balancer op err %v\n", err)
			return err
		}
	}
	return nil
}

type Ctl struct {
	fs.Inode
	bl *Balancer
}

func makeCtl(ctx fs.CtxI, parent fs.Dir, bl *Balancer) fs.Inode {
	i := inode.MakeInode(ctx, sp.DMDEVICE, parent)
	return &Ctl{i, bl}
}

// XXX call balance() repeatedly for each server passed in to write
// XXX assumes one client that retries
func (c *Ctl) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, serr.MkErr(serr.TErrInval, words)
	}
	err := c.bl.balance(words[0], words[1])
	if err != nil {
		return 0, err
	}
	return sessp.Tsize(len(b)), nil
}

func (c *Ctl) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	return nil, serr.MkErr(serr.TErrNotSupported, "Read")
}

func (bl *Balancer) monitor() {
	const MS = 1000
	for true {
		select {
		case <-bl.ch:
			bl.mo.done()
			bl.ch <- true
			return
		default:
			time.Sleep(time.Duration(MS) * time.Millisecond)
			bl.mo.doMonitor(bl.conf)
		}
	}
}

// Monitor if i am connected; if not, terminate myself
func (bl *Balancer) monitorMyself() {
	for true {
		time.Sleep(time.Duration(500) * time.Millisecond)
		_, err := readConfig(bl.FsLib, KVConfig(bl.job))
		if err != nil {
			if serr.IsErrUnreachable(err) {
				db.DFatalf("disconnected\n")
			}
		}
	}
}

// Post config atomically
func (bl *Balancer) PostConfig() {
	err := bl.PutFileJsonAtomic(KVConfig(bl.job), 0777, *bl.conf)
	if err != nil {
		db.DFatalf("%v: MakeFile %v err %v\n", proc.GetName(), KVConfig(bl.job), err)
	}
}

// Post new epoch, and finish moving sharddirs.
func (bl *Balancer) restore(conf *Config, epoch sessp.Tepoch) {
	bl.conf = conf
	// Increase epoch, even if the config is the same as before,
	// so that helpers and clerks realize there is new balancer.
	bl.conf.Epoch = epoch
	db.DPrintf(db.KVBAL, "restore to %v with epoch %v\n", bl.conf, epoch)
	bl.PostConfig()
	bl.doMoves(bl.conf.Moves)
}

func (bl *Balancer) recover(epoch sessp.Tepoch) {
	conf, err := readConfig(bl.FsLib, KVConfig(bl.job))
	if err == nil {
		bl.restore(conf, epoch)
	} else {
		// this must be the first recovery of the balancer;
		// otherwise, there would be a either a config or
		// backup config.
		bl.conf = MakeConfig(epoch)
		bl.PostConfig()
	}
}

// Make intial shard directories
func (bl *Balancer) initShards(nextShards []string) {
	for s, kvd := range nextShards {
		dst := kvShardPath(bl.job, kvd, Tshard(s))
		// Mkdir may fail because balancer crashed during config 0
		// so ignore error
		if err := bl.MkDir(dst, 0777); err != nil {
			db.DPrintf(db.KVBAL_ERR, "warning mkdir %v err %v\n", dst, err)
		}
	}
}

func (bl *Balancer) spawnProc(args []string) (proc.Tpid, error) {
	p := proc.MakeProc(args[0], args[1:])
	p.AppendEnv("SIGMACRASH", bl.crashChild)
	err := bl.Spawn(p)
	if err != nil {
		db.DPrintf(db.KVBAL_ERR, "spawn pid %v err %v\n", p.GetPid(), err)
	}
	return p.GetPid(), err
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
			db.DPrintf(db.ALWAYS, "runProc %v err %v status %v\n", args, err, status)
		}
		if err != nil && (strings.HasPrefix(err.Error(), "Spawn error") ||
			strings.HasPrefix(err.Error(), "Missing return status") ||
			serr.IsErrUnreachable(err)) {
			db.DFatalf("CRASH %v: runProc err %v\n", proc.GetName(), err)
		}
		if retryf(err, status) {
			db.DPrintf(db.KVBAL_ERR, "retry %v err %v status %v\n", args, err, status)
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
			shard := Tshard(i)
			s := kvShardPath(bl.job, kvd, shard)
			d := kvShardPath(bl.job, nextShards[i], shard)
			moves = append(moves, &Move{s, d})
		}
	}
	return moves
}

func (bl *Balancer) doMove(ch chan int, m *Move, i int) {
	if m != nil {
		bl.runProcRetry([]string{"kv-mover", bl.job, bl.conf.Epoch.String(), m.Src, m.Dst},
			func(err error, status *proc.Status) bool {
				db.DPrintf(db.KVBAL, "%v: move %v m %v err %v status %v\n", bl.conf.Epoch, i, m, err, status)
				return err != nil || !status.IsStatusOK()
			})
	}

	ch <- i
}

// Perform moves in parallel
func (bl *Balancer) doMoves(moves Moves) {
	todo := make(Moves, len(moves))
	copy(todo, moves)
	ch := make(chan int)
	for i, m := range moves {
		go bl.doMove(ch, m, i)
	}
	m := 0
	for range moves {
		i := <-ch
		bl.conf.Moves[i] = nil
		db.DPrintf(db.KVBAL, "Cleared move %v %v\n", i, bl.conf)
		bl.PostConfig()
		m += 1
	}
	db.DPrintf(db.ALWAYS, "%v: all moves done\n", bl.conf)
}

func (bl *Balancer) balance(opcode, mfs string) *serr.Err {
	if bl.testAndSetIsBusy() {
		return serr.MkErr(serr.TErrRetry, fmt.Sprintf("busy %v", proc.GetName()))
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

	epoch, err := bl.lc.EnterNextEpoch([]string{})
	if err != nil {
		db.DPrintf(db.KVBAL_ERR, "EnterNextEpoch fail %v\n", err)
		var nperr *serr.Err
		if errors.As(err, &nperr) {
			return nperr
		}
		return serr.MkErr(serr.TErrError, err)
	}

	bl.conf.Epoch = epoch
	bl.conf.Shards = nextShards
	bl.conf.Moves = moves

	db.DPrintf(db.ALWAYS, "New config %v\n", bl.conf)

	// If balancer crashes, before here, KVCONFIG has the old
	// config; otherwise, the new conf.
	bl.PostConfig()

	bl.doMoves(moves)

	if docrash { // start crashing?
		crash.Crasher(bl.FsLib)
	}

	return nil
}
