// Package kv implements a sharded KV service.  A KV service
// deployment has several balancers: one primary and several hot
// standbys. The primary balancer acts as a coordinator for a sharded
// KV service.  Clients can ask the balancer to add shards, and the
// primary balancer updates KVCONF, which has the mapping from shards
// to groups.
//
// Applications interact with the kv service using a clerk, which
// provides a Put/Get API and uses KVCONF to find the shard for key.
//
// A shard is implemented by the [kvgrp] package.
//
// If the balancer isn't the primary anymore (e.g., it is partitioned
// and another balancer has become primary), the old primary's writes
// will fail, because its fences have an old epoch.
package kv

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"

	"sigmaos/api/fs"
	"sigmaos/apps/cache"
	"sigmaos/apps/kv/kvgrp"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/sigmasrv/memfssrv/memfs/dir"
	"sigmaos/sigmasrv/memfssrv/memfs/fenceddir"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
	"sigmaos/util/crash"
)

type Balancer struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	conf    *Config
	lc      *leaderclnt.LeaderClnt
	mo      *Monitor
	job     string
	kvdmcpu proc.Tmcpu
	ch      chan bool
	isBusy  bool // in config change?
	kc      *KvClerk
	repl    string
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

func RunBalancer(job, kvdmcpu string, auto string, repl string) {
	bl := &Balancer{}

	// reject requests for changes until after recovery
	bl.isBusy = true

	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt err %v", err)
	}
	bl.SigmaClnt = sc
	bl.job = job
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
		db.DFatalf("NewLeaderClnt %v", err)
	}

	ssrv, err := sigmasrv.NewSigmaSrvClntNoRPC("", bl.SigmaClnt)
	if err != nil {
		db.DFatalf("StartMemFs %v", err)
	}
	ctx := ctx.NewCtx(sp.NewPrincipal(sp.TprincipalID(KVBALANCER), bl.SigmaClnt.ProcEnv().GetRealm()), nil, 0, sp.NoClntId, nil, nil)
	root, _, _ := ssrv.Root(path.Tpathname{})
	err1 := dir.MkNod(ctx, fenceddir.GetDir(root), "ctl", newCtl(ctx, bl))
	if err1 != nil {
		db.DFatalf("MkNod clone failed %v", err1)
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
		db.DFatalf("Marshal failed %v", error)
	}

	if err := bl.lc.LeadAndFence(b, []string{kvgrp.JobDir(bl.job)}); err != nil {
		db.DFatalf("LeadAndFence %v err %v", kvgrp.JobDir(bl.job), err)
	}

	db.DPrintf(db.ALWAYS, "primary %v with fence %v", bl.ProcEnv().GetPID(), bl.lc.Fence())

	if err := bl.MkLeasedEndpoint(KVBalancer(bl.job), ep, bl.lc.Lease()); err != nil {
		db.DFatalf("MkEndpointFile %v at %v err %v", ep, KVBalancer(bl.job), err)
	}

	// first epoch is used to create a functional system (e.g.,
	// creating shards), so don't allow a crash then.
	if _, err := bl.Stat(KVConfig(bl.job)); err == nil {
		crash.FailersDefault(bl.FsLib, []crash.Tselector{crash.KVBALANCER_CRASH, crash.KVBALANCER_PARTITION})
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

	db.DPrintf(db.KVBAL, "terminate")

	if bl.mo != nil {
		bl.ch <- true
		<-bl.ch
	}
	ssrv.SrvExit(proc.NewStatus(proc.StatusEvicted))
}

func BalancerOp(fsl *fslib.FsLib, job string, opcode, kvd string) error {
	s := opcode + " " + kvd
	db.DPrintf(db.KVBAL, "Balancer %v op %v", KVBalancerCtl(job), opcode)
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
			db.DPrintf(db.KVBAL_ERR, "balancer op wait err %v", err)
			time.Sleep(WAITMS * time.Millisecond)
		} else {
			db.DPrintf(db.KVBAL_ERR, "balancer op err %v", err)
			return err
		}
	}
	return nil
}

type Ctl struct {
	fs.Inode
	bl *Balancer
}

func newCtl(ctx fs.CtxI, bl *Balancer) fs.FsObj {
	i := inode.NewInode(ctx, sp.DMDEVICE, sp.NoLeaseId)
	return &Ctl{i, bl}
}

func (c *Ctl) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
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
			crash.Crash()
		}
	}
}

// Post config atomically
func (bl *Balancer) PostConfig() {
	if err := bl.PutFileJsonAtomic(KVConfig(bl.job), 0777, *bl.conf); err != nil {
		if serr.IsErrCode(err, serr.TErrUnreachable) {
			crash.Crash()
		}
		db.DFatalf("NewFile %v err %v", KVConfig(bl.job), err)
	}
}

// Post new epoch, and finish moving sharddirs.
func (bl *Balancer) restore(conf *Config, fence sp.Tfence) {
	bl.conf = conf
	bl.conf.Fence = fence
	bl.conf.Ncoord += 1
	db.DPrintf(db.KVBAL, "restore to %v with fence %v", bl.conf, fence)
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
		db.DPrintf(db.KVBAL, "initshards %v %v", kvd, s)
		srv := kvGrpPath(bl.job, kvd)

		// simulate that the creates happen after posting
		// configuration so that initial kvds start in conf 1, as the
		// clerks do.
		f := bl.conf.Fence
		f.Seqno = 1

		if err := bl.kc.CreateShard(srv, cache.Tshard(s), &f, make(cache.Tcache)); err != nil {
			db.DFatalf("CreateShard %v %d err %v", kvd, s, err)
		}
	}
}

func (bl *Balancer) spawnProc(args []string) (sp.Tpid, error) {
	p := proc.NewProc(args[0], args[1:])
	err := bl.Spawn(p)
	if err != nil {
		db.DPrintf(db.KVBAL_ERR, "spawn pid %v err %v", p.GetPid(), err)
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

func (bl *Balancer) runProcRetry(args []string, retryf func(error, *proc.Status) bool) int64 {
	nretry := int64(0)
	for true {
		pid, status, err := bl.runProc(args)
		if err != nil {
			db.DPrintf(db.ALWAYS, "runProc %v %v err %v status %v", pid, args, err, status)
		}
		if err != nil && (strings.HasPrefix(err.Error(), "Spawn error") ||
			strings.HasPrefix(err.Error(), "Missing return status") ||
			serr.IsErrCode(err, serr.TErrUnreachable)) {
			db.DFatalf("CRASH: runProc %v err %v", pid, err)
		}
		if retryf(err, status) {
			db.DPrintf(db.KVBAL_ERR, "retry pid %v %v err %v status %v", pid, args, err, status)
		} else {
			break
		}
		nretry += 1
	}
	return nretry
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

type moveRes struct {
	i      int
	nretry int64
	res    TmoverRes
}

func (bl *Balancer) doMove(ch chan moveRes, m *Move, i int) {
	nr := int64(0)
	mr := TmoverRes{}
	if m != nil {
		nr = bl.runProcRetry([]string{"kv-mover", bl.job, string(bl.conf.Fence.Json()), strconv.Itoa(int(m.Shard)), m.Src, m.Dst, bl.repl},
			func(err error, status *proc.Status) bool {
				db.DPrintf(db.KVBAL, "%v: move %v m %v err %v status %v", bl.conf.Fence.Epoch, i, m, err, status)
				if err == nil && status.IsStatusOK() {
					mapstructure.Decode(status.Data(), &mr)
				}
				return err != nil || !status.IsStatusOK()
			})
	}

	ch <- moveRes{i, nr, mr}
}

// Perform moves in parallel
func (bl *Balancer) doMoves(moves Moves) {
	todo := make(Moves, len(moves))
	copy(todo, moves)
	ch := make(chan moveRes)
	for i, m := range moves {
		go bl.doMove(ch, m, i)
	}
	m := 0
	for range moves {
		mr := <-ch
		bl.conf.Moves[mr.i] = nil
		db.DPrintf(db.KVBAL, "Cleared move %v %v", mr, bl.conf)
		bl.conf.Nmovers += 1
		bl.conf.Nretry += mr.nretry
		bl.conf.MovMs += mr.res.Ms
		bl.conf.Nkeys += mr.res.Nkeys

		bl.PostConfig()
		m += 1
	}
	db.DPrintf(db.ALWAYS, "%v: all moves done", bl.conf)
}

func (bl *Balancer) balance(opcode, kvd string) *serr.Err {
	if bl.testAndSetIsBusy() {
		return serr.NewErr(serr.TErrRetry, fmt.Sprintf("busy %v", bl.ProcEnv().GetPID()))
	}
	defer bl.clearIsBusy()

	db.DPrintf(db.KVBAL, "%v: opcode %v kvd %v conf %v", bl.ProcEnv().GetPID(), opcode, kvd, bl.conf)

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

	db.DPrintf(db.ALWAYS, "New config %v", bl.conf)

	// If balancer crashes, before here, KVCONFIG has the old
	// config; otherwise, the new conf.
	bl.PostConfig()

	bl.doMoves(moves)

	if docrash { // start crashing?
		crash.FailersDefault(bl.FsLib, []crash.Tselector{crash.KVBALANCER_CRASH, crash.KVBALANCER_PARTITION})
	}

	return nil
}
