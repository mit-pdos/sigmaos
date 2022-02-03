package kv

//
// A balancer, which acts as a coordinator for the sharded KV service.
// A KV service deployment may have several balancer: one primary and
// several backups.
//

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"ulambda/atomic"
	"ulambda/crash"
	"ulambda/ctx"
	db "ulambda/debug"
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
	KVDIR         = "name/kv"
	KVCONFIG      = KVDIR + "/config"      // ephemeral file with current config
	KVNEXTCONFIG  = KVDIR + "/nextconfig"  // the persistent next configuration
	KVNEXTBK      = KVDIR + "/nextconfig#" // temporary copy
	KVBALANCER    = KVDIR + "/balancer"
	KVBALANCERCTL = KVDIR + "/balancer/ctl"
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
	log.Printf("%v: balancer crash %v crashhelper %v auto %v\n", proc.GetPid(), crash.GetEnv(), crashhelper, auto)

	bl := &Balancer{}
	bl.FsLib = fslib.MakeFsLib("balancer-" + proc.GetPid())
	bl.ProcClnt = procclnt.MakeProcClnt(bl.FsLib)
	bl.crash = crash.GetEnv()
	bl.crashhelper = crashhelper

	// may fail if already exist
	// bl.Mkdir(np.MEMFS, 07)
	bl.Mkdir(KVDIR, 07)

	bl.balFclnt = fenceclnt.MakeFenceClnt(bl.FsLib, KVBALANCER, np.DMSYMLINK)
	bl.confFclnt = fenceclnt.MakeFenceClnt(bl.FsLib, KVCONFIG, 0)

	bl.setRecovering(true)

	// start server but don't publish its existence
	mfs, _, err := fslibsrv.MakeMemFs("", "balancer-"+proc.GetPid())
	if err != nil {
		log.Fatalf("FATAL StartMemFs %v\n", err)
	}
	ctx := ctx.MkCtx("balancer", 0, nil)
	err = dir.MkNod(ctx, mfs.Root(), "ctl", makeCtl(ctx, mfs.Root(), bl))
	if err != nil {
		log.Fatalf("FATAL MakeNod clone failed %v\n", err)
	}

	// start server and write ch when server is done
	ch := make(chan bool)
	go func() {
		mfs.Serve()
		ch <- true
	}()

	err = bl.balFclnt.AcquireFenceW(fslib.MakeTarget(mfs.MyAddr()))
	if err != nil {
		log.Fatalf("FATAL %v: AcquireFenceW %v\n", db.GetName(), err)
	}

	log.Printf("%v: primary\n", db.GetName())

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
	err := fsl.WriteFile(KVBALANCERCTL, []byte(s))
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
func (c *Ctl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, fmt.Errorf("Invalid arguments")
	}
	err := c.bl.balance(words[0], words[1])
	return np.Tsize(len(b)), err
}

func (c *Ctl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	return nil, nil
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
			if err.Error() == "EOF" {
				// we are disconnected
				log.Printf("%v: monitorMyself err %v\n", db.GetName(), err)
				ch <- true
			}
		}
	}
}

func (bl *Balancer) PublishConfig() {
	err := bl.Remove(KVNEXTBK)
	if err != nil {
		log.Printf("%v: Remove %v err %v\n", db.GetName(), KVNEXTBK, err)
	}
	err = atomic.MakeFileJsonAtomic(bl.FsLib, KVNEXTBK, 0777, *bl.conf)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFile %v err %v\n", db.GetName(), KVNEXTBK, err)
	}
	err = bl.confFclnt.MakeFenceFileFrom(KVNEXTBK)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFenceFileFrom err %v\n", db.GetName(), err)
	}
}

// Restore sharddirs by finishing moves and deletes, and create config
// file.   XXX we could restore in parallel with serving clerk ops
func (bl *Balancer) restore() {
	// Increase epoch, even if the config is the same as before,
	// so that helpers and clerks realize there is new balancer.
	bl.conf.N += 1
	log.Printf("%v: restore to %v\n", db.GetName(), bl.conf)

	// first republish next config, which we just read into
	// bl.conf, to obtain the kvconfig fence and bump its seqno
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
			log.Printf("%v: warning mkdir %v err %v\n", db.GetName(), dst, err)
		}
	}
}

func (bl *Balancer) spawnProc(args []string) (string, error) {
	p := proc.MakeProc(args[0], args[1:])
	log.Printf("%v: spawn pid %v %v\n", db.GetName(), p.Pid, bl.crashhelper)
	p.AppendEnv("SIGMACRASH", bl.crashhelper)
	err := bl.Spawn(p)
	if err != nil {
		log.Printf("%v: spawn pid %v err %v\n", db.GetName(), p.Pid, err)
	}
	return p.Pid, err
}

func (bl *Balancer) runProc(args []string) (string, error) {
	pid, err := bl.spawnProc(args)
	if err != nil {
		return "", err
	}
	log.Printf("%v: proc %v wait %v\n", db.GetName(), args, pid)
	status, err := bl.WaitExit(pid)
	return status, err
}

func (bl *Balancer) runProcRetry(args []string, retryf func(error, string) bool) (error, string) {
	status := ""
	var err error
	for true {
		status, err = bl.runProc(args)
		if err != nil {
			log.Printf("%v: runProc %v err %v status %v\n", db.GetName(), args, err, status)
		}
		if err != nil && (strings.HasPrefix(err.Error(), "Spawn error") ||
			strings.HasPrefix(err.Error(), "Missing return status") ||
			strings.HasPrefix(err.Error(), "EOF")) {
			log.Fatalf("FATAL %v: runProc err %v\n", db.GetName(), err)
		}
		if retryf(err, status) {
			log.Printf("%v: retry proc %v err %v status %v\n", db.GetName(), args, err, status)
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
	log.Printf("%v: check moves %v\n", db.GetName(), len(moves))
	for _, m := range moves {
		fn := m.Dst
		_, err := bl.Stat(fn)
		if err != nil {
			log.Printf("%v: fn isn't there %v\n", db.GetName(), fn)
		}
	}
}

// Run deleters in parallel
func (bl *Balancer) runDeleters(moves Moves) {
	log.Printf("%v: start deleting %v\n", db.GetName(), len(moves))
	tmp := make(Moves, len(moves))
	ch := make(chan int)
	for i, m := range moves {
		go func(m *Move, i int) {
			err, status := bl.runProcRetry([]string{"bin/user/kv-deleter", strconv.Itoa(bl.conf.N), m.Src},
				func(err error, status string) bool {
					ok := strings.HasPrefix(status, "file not found")
					return err != nil || (status != "OK" && !ok)
				})
			log.Printf("%v: delete %v/%v done err %v status %v\n", db.GetName(), i, m, err, status)
			ch <- i
		}(m, i)
	}
	m := 0
	for range moves {
		i := <-ch
		tmp[i] = nil
		m += 1
		log.Printf("%v: deleter done %v %v\n", db.GetName(), m, tmp)
	}
	log.Printf("%v: deleters done\n", db.GetName())
}

// Run movers in parallel
func (bl *Balancer) runMovers(moves Moves) {
	log.Printf("%v: start moving %v\n", db.GetName(), len(moves))
	tmp := make(Moves, len(moves))
	copy(tmp, moves)
	ch := make(chan int)
	for i, m := range moves {
		go func(m *Move, i int) {
			err, status := bl.runProcRetry([]string{"bin/user/kv-mover", strconv.Itoa(bl.conf.N), m.Src, m.Dst}, func(err error, status string) bool {
				return err != nil || status != "OK"
			})
			log.Printf("%v: move %v m %v done err %v status %v\n", db.GetName(), i, m, err, status)
			ch <- i
		}(m, i)
	}
	m := 0
	for range moves {
		i := <-ch
		tmp[i] = nil
		m += 1
		log.Printf("%v: movers done %v %v\n", db.GetName(), m, tmp)
	}
	log.Printf("%v: movers all done\n", db.GetName())
}

func (bl *Balancer) balance(opcode, mfs string) error {
	if bl.testAndSetRecovering() {
		log.Printf("%v: force client to retry\n", db.GetName())
		return fmt.Errorf("retry")
	}
	defer bl.setRecovering(false)

	log.Printf("%v: BAL Balancer: %v %v %v\n", db.GetName(), opcode, mfs, bl.conf)

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

	log.Printf("%v: BAL conf %v next shards: %v\n", db.GetName(), bl.conf, nextShards)

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

	log.Printf("%v: new %v\n", db.GetName(), bl.conf)

	// If balancer crashes, before here, we have the old
	// KVNEXTCONFIG.  If the balancer crash after, we have the new
	// KVNEXTCONFIG.
	err := atomic.MakeFileJsonAtomic(bl.FsLib, KVNEXTCONFIG, 0777, *bl.conf)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFile %v err %v\n", db.GetName(), KVNEXTCONFIG, err)
	}

	// Announce new KVNEXTCONFIG to world: copy KVNEXTCONFIG to
	// KVNEXTBK and make fence from copy (removing the copy too).
	bl.PublishConfig()

	bl.runMovers(moves)

	bl.checkMoves(moves)

	bl.runDeleters(moves)

	return nil
}
