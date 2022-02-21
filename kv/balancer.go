package kv

//
// A balancer, which acts as a coordinator for the sharded KV service.
// A KV service deployment may have several balancer: one primary and
// several backups.
//

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"ulambda/atomic"
	"ulambda/crash"
	"ulambda/ctx"
	"ulambda/dir"
	"ulambda/fenceclnt"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/group"
	"ulambda/inode"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	NKV           = 10
	NSHARD        = 10 * NKV
	KVDIR         = "name/kv/"
	KVCONF        = "config"
	KVCONFIG      = KVDIR + KVCONF        // file with current config
	KVNEXTCONFIG  = KVDIR + "nextconfig"  // the persistent next configuration
	KVNEXTBK      = KVDIR + "nextconfig#" // temporary copy
	KVBALANCER    = KVDIR + "balancer"
	KVBALANCERCTL = KVDIR + "balancer/ctl"
)

type Balancer struct {
	sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	conf         *Config
	balFclnt     *fenceclnt.FenceClnt
	confFclnt    *fenceclnt.FenceClnt
	mo           *Monitor
	ch           chan bool
	crash        int64
	crashhelper  string
	isRecovering bool
}

func (bl *Balancer) testAndSetRecovering() bool {
	bl.Lock()
	defer bl.Unlock()
	b := bl.isRecovering
	if !bl.isRecovering {
		bl.isRecovering = true
	}
	return b
}

func (bl *Balancer) setRecovering(b bool) {
	bl.Lock()
	defer bl.Unlock()
	bl.isRecovering = b
}

func shardPath(kvd, shard string) string {
	return group.GRPDIR + "/" + kvd + "/shard" + shard
}

func RunBalancer(crashhelper string, auto string) {
	bl := &Balancer{}
	bl.FsLib = fslib.MakeFsLib("balancer-" + proc.GetPid())
	bl.ProcClnt = procclnt.MakeProcClnt(bl.FsLib)
	bl.crash = crash.GetEnv()
	bl.crashhelper = crashhelper

	// may fail if already exist
	// bl.Mkdir(np.MEMFS, 07)
	bl.Mkdir(KVDIR, 07)

	srvs := []string{KVDIR}
	bl.balFclnt = fenceclnt.MakeFenceClnt(bl.FsLib, KVBALANCER, np.DMSYMLINK, srvs)
	bl.confFclnt = fenceclnt.MakeFenceClnt(bl.FsLib, KVCONFIG, 0, srvs)

	bl.setRecovering(true)

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

	err = bl.balFclnt.AcquireFenceW(fslib.MakeTarget([]string{mfs.MyAddr()}))
	if err != nil {
		log.Fatalf("FATAL %v: AcquireFenceW %v\n", proc.GetName(), err)
	}

	log.Printf("%v: primary\n", proc.GetName())

	select {
	case <-ch:
		// done
	default:
		bl.recover()

		bl.setRecovering(false)

		go bl.monitorMyself(ch)

		crash.Crasher(bl.FsLib)

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
	_, err := fsl.SetFile(KVBALANCERCTL, []byte(s), 0)
	return err
}

type Ctl struct {
	fs.FsObj
	bl *Balancer
}

func makeCtl(ctx fs.CtxI, parent fs.Dir, bl *Balancer) fs.FsObj {
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
			if np.IsErrEOF(err) {
				// we are disconnected
				// log.Printf("%v: monitorMyself err %v\n", proc.GetName(), err)
				ch <- true
			}
		}
	}
}

// Publish new KVCONFIG through OpenFenceFrom(). Afterwards our writes
// to the server holding KVCONFIG are fenced with a new fence.
func (bl *Balancer) PublishConfig() {
	err := bl.Remove(KVNEXTBK)
	if err != nil {
		log.Printf("%v: Remove %v err %v\n", proc.GetName(), KVNEXTBK, err)
	}
	err = atomic.PutFileJsonAtomic(bl.FsLib, KVNEXTBK, 0777, *bl.conf)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFile %v err %v\n", proc.GetName(), KVNEXTBK, err)
	}
	err = bl.confFclnt.OpenFenceFrom(KVNEXTBK)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFenceFileFrom err %v\n", proc.GetName(), err)
	}
}

// Restore sharddirs by finishing moves and deletes, and create config
// file.
func (bl *Balancer) restore() {
	// Increase epoch, even if the config is the same as before,
	// so that helpers and clerks realize there is new balancer.
	bl.conf.N += 1
	log.Printf("%v: restore to %v\n", proc.GetName(), bl.conf)

	// first republish next config, which the caller read into
	// bl.conf, to obtain the kvconfig fence and bump its seqno.
	bl.PublishConfig()
	bl.runMovers(bl.conf.Moves)
	bl.runDeleters(bl.conf.Moves)
}

func (bl *Balancer) recover() {
	var err error
	bl.conf, err = readConfig(bl.FsLib, KVNEXTCONFIG)
	if err == nil {
		bl.restore()
	} else {
		// this must be the first recovery of the balancer;
		// otherwise, there would be a either a config or
		// backup config.
		bl.conf = MakeConfig(0)
		bl.PublishConfig()
	}
}

// Make intial shard directories
func (bl *Balancer) initShards(nextShards []string) {
	for s, kvd := range nextShards {
		dst := shardPath(kvd, strconv.Itoa(s))
		// Mkdir may fail because balancer crashed during config 0
		// so ignore error
		if err := bl.Mkdir(dst, 0777); err != nil {
			log.Printf("%v: warning mkdir %v err %v\n", proc.GetName(), dst, err)
		}
	}
}

func (bl *Balancer) spawnProc(args []string) (string, error) {
	p := proc.MakeProc(args[0], args[1:])
	// log.Printf("%v: spawn pid %v %v\n", proc.GetName(), p.Pid, bl.crashhelper)
	p.AppendEnv("SIGMACRASH", bl.crashhelper)
	err := bl.Spawn(p)
	if err != nil {
		log.Printf("%v: spawn pid %v err %v\n", proc.GetName(), p.Pid, err)
	}
	return p.Pid, err
}

func (bl *Balancer) runProc(args []string) (*proc.Status, error) {
	pid, err := bl.spawnProc(args)
	if err != nil {
		return nil, err
	}
	// log.Printf("%v: proc %v wait %v\n", proc.GetName(), args, pid)
	status, err := bl.WaitExit(pid)
	return status, err
}

func (bl *Balancer) runProcRetry(args []string, retryf func(error, *proc.Status) bool) (error, *proc.Status) {
	var status *proc.Status
	var err error
	for true {
		status, err = bl.runProc(args)
		if err != nil {
			log.Printf("%v: runProc %v err %v status %v\n", proc.GetName(), args, err, status)
		}
		if err != nil && (strings.HasPrefix(err.Error(), "Spawn error") ||
			strings.HasPrefix(err.Error(), "Missing return status") ||
			np.IsErrEOF(err)) {
			log.Fatalf("CRASH %v: runProc err %v\n", proc.GetName(), err)
		}
		if retryf(err, status) {
			log.Printf("%v: retry proc %v err %v status %v\n", proc.GetName(), args, err, status)
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
			shard := strconv.Itoa(i)
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
			log.Printf("%v: fn isn't there %v\n", proc.GetName(), fn)
		}
	}
}

// Run deleters in parallel
func (bl *Balancer) runDeleters(moves Moves) {
	tmp := make(Moves, len(moves))
	ch := make(chan int)
	for i, m := range moves {
		go func(m *Move, i int) {
			bl.runProcRetry([]string{"bin/user/kv-deleter", strconv.Itoa(bl.conf.N), m.Src},
				func(err error, status *proc.Status) bool {
					ok := strings.HasPrefix(status.Msg(), "file not found")
					return err != nil || (!status.IsStatusOK() && !ok)
				})
			// log.Printf("%v: delete %v/%v done err %v status %v\n", proc.GetName(), i, m, err, status)
			ch <- i
		}(m, i)
	}
	m := 0
	for range moves {
		i := <-ch
		tmp[i] = nil
		m += 1
		// log.Printf("%v: deleter done %v %v\n", proc.GetName(), m, tmp)
	}
	log.Printf("%v: deleters done\n", proc.GetName())
}

// Run movers in parallel
func (bl *Balancer) runMovers(moves Moves) {
	tmp := make(Moves, len(moves))
	copy(tmp, moves)
	ch := make(chan int)
	for i, m := range moves {
		go func(m *Move, i int) {
			bl.runProcRetry([]string{"bin/user/kv-mover", strconv.Itoa(bl.conf.N), m.Src, m.Dst}, func(err error, status *proc.Status) bool {
				return err != nil || !status.IsStatusOK()
			})
			// log.Printf("%v: move %v m %v done err %v status %v\n", proc.GetName(), i, m, err, status)
			ch <- i
		}(m, i)
	}
	m := 0
	for range moves {
		i := <-ch
		tmp[i] = nil
		m += 1
		// log.Printf("%v: mover done %v %v\n", proc.GetName(), m, tmp)
	}
	log.Printf("%v: movers all done\n", proc.GetName())
}

func (bl *Balancer) balance(opcode, mfs string) *np.Err {
	if bl.testAndSetRecovering() {
		return np.MkErr(np.TErrRetry, "busy")
	}
	defer bl.setRecovering(false)

	// log.Printf("%v: BAL Balancer: %v %v %v\n", proc.GetName(), opcode, mfs, bl.conf)

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

	// log.Printf("%v: BAL conf %v next shards: %v\n", proc.GetName(), bl.conf, nextShards)

	var moves Moves
	if bl.conf.N == 0 {
		bl.initShards(nextShards)
	} else {
		moves = bl.computeMoves(nextShards)
	}

	bl.conf.N += 1
	bl.conf.Shards = nextShards
	bl.conf.Moves = moves
	bl.conf.Ctime = time.Now().UnixNano()

	log.Printf("%v: new %v\n", proc.GetName(), bl.conf)

	// If balancer crashes, before here, we have the old
	// KVNEXTCONFIG.  If the balancer crash after, we have the new
	// KVNEXTCONFIG.
	err := atomic.PutFileJsonAtomic(bl.FsLib, KVNEXTCONFIG, 0777, *bl.conf)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFile %v err %v\n", proc.GetName(), KVNEXTCONFIG, err)
	}

	// Announce new KVNEXTCONFIG to world: copy KVNEXTCONFIG to
	// KVNEXTBK and make fence from copy (removing the copy too).
	bl.PublishConfig()

	bl.runMovers(moves)

	bl.checkMoves(moves)

	bl.runDeleters(moves)

	return nil
}
