package kv

//
// A balancer, which acts as a coordinator for a sharded KV service.
// A KV service deployment has several balancers: one primary and
// several backups.
//
// When a client adds/removes a shard, the primary balancer updates
// KVCONF, which has the mapping from shards to groups.
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

	"sigmaos/cache"
	"sigmaos/cachesrv"
	"sigmaos/crash"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/inode"
	"sigmaos/kvgrp"
	"sigmaos/leaderclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type Balancer struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	conf        *Config
	lc          *leaderclnt.LeaderClnt
	mo          *Monitor
	job         string
	kvdmcpu     proc.Tmcpu
	ch          chan bool
	crashhelper int64
	isBusy      bool // in config change?
	kc          *KvClerk
	repl        string
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

func RunBalancer(job, crashhelperstr, kvdmcpu string, auto string, repl string) {
	bl := &Balancer{}

	// reject requests for changes until after recovery
	bl.isBusy = true

	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt err %v", err)
	}
	bl.SigmaClnt = sc
	bl.job = job
	crashhelper, err := strconv.Atoi(crashhelperstr)
	if err != nil {
		db.DFatalf("Error atoi crashhelperstr: %v", err)
	}
	bl.crashhelper = int64(crashhelper)
	bl.kc = NewClerkFsLib(sc.FsLib, job, repl == "repl")
	bl.repl = repl

	var kvdnc int
	var error error
	kvdnc, error = strconv.Atoi(kvdmcpu)
	if error != nil {
		db.DFatalf("Bad kvdmcpu: %v", error)
	}
	bl.kvdmcpu = proc.Tmcpu(kvdnc)

	bl.lc, err = leaderclnt.NewLeaderClnt(bl.FsLib, KVBalancerElect(bl.job), sp.DMSYMLINK|077)
	if err != nil {
		db.DFatalf("NewLeaderClnt %v\n", err)
	}

	ssrv, err := sigmasrv.NewSigmaSrvClntNoRPC("", bl.SigmaClnt)
	if err != nil {
		db.DFatalf("StartMemFs %v\n", err)
	}
	ctx := ctx.NewCtx(sp.NewPrincipal(sp.TprincipalID(KVBALANCER), bl.SigmaClnt.ProcEnv().GetRealm(), sp.NoToken()), nil, 0, sp.NoClntId, nil, nil)
	root, _, _ := ssrv.Root(path.Path{})
	err1 := dir.MkNod(ctx, root, "ctl", newCtl(ctx, root, bl))
	if err1 != nil {
		db.DFatalf("MkNod clone failed %v\n", err1)
	}

	// start server and write ch when server is done
	ch := make(chan bool)
	go func() {
		ssrv.Serve()
		ch <- true
	}()

	ep := ssrv.GetEndpoint()
	b, error := ep.Marshal()
	if error != nil {
		db.DFatalf("Marshal failed %v\n", error)
	}

	if err := bl.lc.LeadAndFence(b, []string{kvgrp.JobDir(bl.job)}); err != nil {
		db.DFatalf("LeadAndFence %v err %v\n", kvgrp.JobDir(bl.job), err)
	}

	db.DPrintf(db.ALWAYS, "primary %v with fence %v\n", bl.ProcEnv().GetPID(), bl.lc.Fence())

	if err := bl.MkEndpointFile(KVBalancer(bl.job), ep, bl.lc.Lease()); err != nil {
		db.DFatalf("MkEndpointFile %v at %v err %v\n", ep, KVBalancer(bl.job), err)
	}

	// first epoch is used to create a functional system (e.g.,
	// creating shards), so don't allow a crash then.
	if _, err := bl.Stat(KVConfig(bl.job)); err == nil {
		crash.Crasher(bl.FsLib)
	}

	go bl.monitorMyself()

	select {
	case <-ch:
		// done
	default:
		bl.recover(bl.lc.Fence())

		bl.clearIsBusy()

		if auto == "auto" {
			bl.mo = NewMonitor(bl.SigmaClnt, bl.job, bl.kvdmcpu)
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
	ssrv.SrvExit(proc.NewStatus(proc.StatusEvicted))
}

func BalancerOp(fsl *fslib.FsLib, job string, opcode, kvd string) error {
	s := opcode + " " + kvd
	db.DPrintf(db.KVBAL, "Balancer %v op %v\n", KVBalancerCtl(job), opcode)
	_, err := fsl.SetFile(KVBalancerCtl(job), []byte(s), sp.OWRITE, 0)
	return err
}

// Retry a balancer op until success, or an unexpected error is returned.
func BalancerOpRetry(fsl *fslib.FsLib, job, opcode, kvd string) error {
	for true {
		err := BalancerOp(fsl, job, opcode, kvd)
		if err == nil {
			return nil
		}
		var serr *serr.Err
		if errors.As(err, &serr) && (serr.IsErrUnavailable() || serr.IsErrRetry()) {
			db.DPrintf(db.KVBAL_ERR, "balancer op wait err %v\n", err)
			time.Sleep(WAITMS * time.Millisecond)
		} else {
			db.DPrintf(db.KVBAL_ERR, "balancer op err %v\n", err)
			return err
		}
	}
	return nil
}

type Ctl struct {
	fs.Inode
	bl *Balancer
}

func newCtl(ctx fs.CtxI, parent fs.Dir, bl *Balancer) fs.FsObj {
	i := inode.NewInode(ctx, sp.DMDEVICE, parent)
	return &Ctl{i, bl}
}

func (c *Ctl) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	st, err := c.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	return st, nil
}

// XXX call balance() repeatedly for each server passed in to write
// XXX assumes one client that retries
func (c *Ctl) Write(ctx fs.CtxI, off sp.Toffset, b []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, serr.NewErr(serr.TErrInval, words)
	}
	err := c.bl.balance(words[0], words[1])
	if err != nil {
		return 0, err
	}
	return sp.Tsize(len(b)), nil
}

func (c *Ctl) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	return nil, serr.NewErr(serr.TErrNotSupported, "Read")
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

// Monitor if i am connected; if not, terminate myself.  Another
// balancer will take over.  XXX replace by checking if leaderclnt's
// session lease is still valid.
func (bl *Balancer) monitorMyself() {
	for true {
		time.Sleep(time.Duration(500) * time.Millisecond)
		_, err := readConfig(bl.FsLib, KVConfig(bl.job))
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			crash.Crash(0)
		}
	}
}

// Post config atomically
func (bl *Balancer) PostConfig() {
	if err := bl.PutFileJsonAtomic(KVConfig(bl.job), 0777, *bl.conf); err != nil {
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			crash.Crash(0)
		}
		db.DFatalf("NewFile %v err %v\n", KVConfig(bl.job), err)
	}
}

// Post new epoch, and finish moving sharddirs.
func (bl *Balancer) restore(conf *Config, fence sp.Tfence) {
	bl.conf = conf
	bl.conf.Fence = fence
	db.DPrintf(db.KVBAL, "restore to %v with fence %v\n", bl.conf, fence)
	bl.PostConfig()
	bl.doMoves(bl.conf.Moves)
}

func (bl *Balancer) recover(fence sp.Tfence) {
	conf, err := readConfig(bl.FsLib, KVConfig(bl.job))
	if err == nil {
		bl.restore(conf, fence)
	} else {
		// this must be the first recovery of the balancer;
		// otherwise, there would be a either a config or
		// backup config.
		bl.conf = NewConfig(fence)
		bl.PostConfig()
	}
}

// Make intial shard directories
func (bl *Balancer) initShards(nextShards []string) {
	for s, kvd := range nextShards {
		db.DPrintf(db.KVBAL, "initshards %v %v\n", kvd, s)
		srv := kvGrpPath(bl.job, kvd)

		// simulate that the creates happen after posting
		// configuration so that initial kvds start in conf 1, as the
		// clerks do.
		f := bl.conf.Fence
		f.Seqno = 1

		if err := bl.kc.CreateShard(srv, cache.Tshard(s), &f, make(cachesrv.Tcache)); err != nil {
			db.DFatalf("CreateShard %v %d err %v\n", kvd, s, err)
		}
	}
}

func (bl *Balancer) spawnProc(args []string) (sp.Tpid, error) {
	p := proc.NewProc(args[0], args[1:])
	p.SetCrash(bl.crashhelper)
	err := bl.Spawn(p)
	if err != nil {
		db.DPrintf(db.KVBAL_ERR, "spawn pid %v err %v\n", p.GetPid(), err)
	}
	return p.GetPid(), err
}

func (bl *Balancer) runProc(args []string) (sp.Tpid, *proc.Status, error) {
	pid, err := bl.spawnProc(args)
	if err != nil {
		return "", nil, err
	}
	status, err := bl.WaitExit(pid)
	return pid, status, err
}

func (bl *Balancer) runProcRetry(args []string, retryf func(error, *proc.Status) bool) (error, *proc.Status) {
	var status *proc.Status
	var err error
	var pid sp.Tpid
	for true {
		pid, status, err = bl.runProc(args)
		if err != nil {
			db.DPrintf(db.ALWAYS, "runProc %v %v err %v status %v\n", pid, args, err, status)
		}
		if err != nil && (strings.HasPrefix(err.Error(), "Spawn error") ||
			strings.HasPrefix(err.Error(), "Missing return status") ||
			serr.IsErrCode(err, serr.TErrUnreachable)) {
			db.DFatalf("CRASH: runProc %v err %v\n", pid, err)
		}
		if retryf(err, status) {
			db.DPrintf(db.KVBAL_ERR, "retry pid %v %v err %v status %v\n", pid, args, err, status)
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
			s := kvGrpPath(bl.job, kvd)
			d := kvGrpPath(bl.job, nextShards[i])
			moves = append(moves, &Move{cache.Tshard(i), s, d})
		}
	}
	return moves
}

func (bl *Balancer) doMove(ch chan int, m *Move, i int) {
	if m != nil {
		bl.runProcRetry([]string{"kv-mover", bl.job, string(bl.conf.Fence.Json()), strconv.Itoa(int(m.Shard)), m.Src, m.Dst, bl.repl},
			func(err error, status *proc.Status) bool {
				db.DPrintf(db.KVBAL, "%v: move %v m %v err %v status %v\n", bl.conf.Fence.Epoch, i, m, err, status)
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

func (bl *Balancer) balance(opcode, kvd string) *serr.Err {
	if bl.testAndSetIsBusy() {
		return serr.NewErr(serr.TErrRetry, fmt.Sprintf("busy %v", bl.ProcEnv().GetPID()))
	}
	defer bl.clearIsBusy()

	db.DPrintf(db.KVBAL, "%v: opcode %v kvd %v conf %v\n", bl.ProcEnv().GetPID(), opcode, kvd, bl.conf)

	var nextShards []string
	switch opcode {
	case "add":
		if bl.conf.Present(kvd) {
			return nil
		}
		nextShards = AddKv(bl.conf, kvd)
	case "del":
		if !bl.conf.Present(kvd) {
			return nil
		}
		nextShards = DelKv(bl.conf, kvd)
	default:
	}

	var moves Moves
	docrash := false
	if bl.conf.Shards[0] == "" { // first conf
		bl.initShards(nextShards)
		docrash = true
	} else {
		moves = bl.computeMoves(nextShards)
	}

	bl.conf.Fence.Seqno += 1
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
