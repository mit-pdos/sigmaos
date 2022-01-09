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
	"time"

	"ulambda/atomic"
	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/group"
	"ulambda/inode"
	"ulambda/leaseclnt"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	NKV           = 10
	NSHARD        = 10 * NKV
	KVDIR         = "name/kv"
	KVCONFIG      = KVDIR + "/config"
	KVCONFIGBK    = KVDIR + "/config#"
	KVNEXTCONFIG  = KVDIR + "/nextconfig"
	KVBALANCER    = KVDIR + "/balancer"
	KVBALANCERCTL = KVDIR + "/balancer/ctl"

	CRASHHELPER = 10
)

type Balancer struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	conf     *Config
	balLease *leaseclnt.LeaseClnt
	lease    *leaseclnt.LeaseClnt
	mo       *Monitor
	ch       chan bool
	crash    int64
	isBusy   bool
}

func shardPath(kvd, shard string) string {
	return group.GRPDIR + "/" + kvd + "/shard" + shard
}

func RunBalancer(auto string) {
	log.Printf("run balancer %v %v\n", proc.GetPid(), auto)

	bl := &Balancer{}
	bl.FsLib = fslib.MakeFsLib("balancer-" + proc.GetPid())
	bl.ProcClnt = procclnt.MakeProcClnt(bl.FsLib)
	bl.crash = crash.GetEnv()

	// may fail if already exist
	// bl.Mkdir(np.MEMFS, 07)
	bl.Mkdir(KVDIR, 07)

	bl.balLease = leaseclnt.MakeLeaseClnt(bl.FsLib, KVBALANCER, np.DMSYMLINK)
	bl.lease = leaseclnt.MakeLeaseClnt(bl.FsLib, KVCONFIG, 0)
	bl.isBusy = true

	// start server but don't publish its existence
	mfs, _, err := fslibsrv.MakeMemFs("", "balancer-"+proc.GetPid())
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	err = dir.MkNod(fssrv.MkCtx(""), mfs.Root(), "ctl", makeCtl("balancer", mfs.Root(), bl))
	if err != nil {
		log.Fatalf("MakeNod clone failed %v\n", err)
	}

	// start server and write ch when server is done
	ch := make(chan bool)
	go func() {
		mfs.Serve()
		ch <- true
	}()

	bl.balLease.WaitWLease(fslib.MakeTarget(mfs.MyAddr()))

	log.Printf("%v: primary\n", db.GetName())

	select {
	case <-ch:
		// done
	default:
		bl.recover()

		bl.isBusy = false

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

func makeCtl(uname string, parent fs.Dir, bl *Balancer) fs.FsObj {
	i := inode.MakeInode(uname, np.DMDEVICE, parent)
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
			if err.Error() == "EOF" ||
				strings.HasPrefix(err.Error(), "stale lease") {
				// we are disconnected
				//log.Printf("%v: monitorMyself err %v\n", db.GetName(), err)

				ch <- true
			}
		}
	}
}

// Remove shardirs that balancer didn't get to because it crashed
// after committing to new config, but before deleting the moved
// shards.
func (bl *Balancer) cleanup() {
	log.Printf("cleanup %v\n", bl.conf.Moved)
	bl.runDeleters(bl.conf.Moved)
}

func (bl *Balancer) recover() {
	var err error
	bl.conf, err = readConfig(bl.FsLib, KVCONFIG)
	if err == nil {
		log.Printf("%v: recovery: use %v\n", db.GetName(), bl.conf)
		bl.cleanup()
		return
	}

	// roll back to old conf
	err = bl.lease.MakeLeaseFileFrom(KVCONFIGBK)
	if err != nil {
		log.Printf("%v: MakeLeaseFileFrom %v err %v\n", db.GetName(), KVCONFIGBK, err)
		// this must be the first recovery of the balancer;
		// otherwise, there would be a either a config or
		// backup config.
		err = bl.lease.MakeLeaseFileJson(*MakeConfig(0))
		if err != nil {
			log.Fatalf("%v: recover failed to create initial config\n", db.GetName())
		}
	} else {
		log.Printf("%v: recovery: restored config from %v\n", db.GetName(), KVCONFIGBK)
	}
}

// Make intial shard directories
func (bl *Balancer) initShards(nextShards []string) {
	for s, kvd := range nextShards {
		dst := shardPath(kvd, strconv.Itoa(s))
		db.DLPrintf("BAL", "Init shard dir %v\n", dst)
		// Mkdir may fail because balancer crashed during config 0
		// so ignore error
		bl.Mkdir(dst, 0777)
	}
}

func (bl *Balancer) spawnProc(args []string) (string, error) {
	p := proc.MakeProc(args[0], args[1:])
	if bl.crash > 0 {
		p.AppendEnv("SIGMACRASH", strconv.Itoa(CRASHHELPER))
	}
	err := bl.Spawn(p)
	return p.Pid, err
}

func (bl *Balancer) runProc(args []string) (string, error) {
	pid, err := bl.spawnProc(args)
	if err != nil {
		return "", err
	}
	status, err := bl.WaitExit(pid)
	return status, err
}

func (bl *Balancer) runProcRetry(args []string, retryf func(error, string) bool) error {
	for true {
		status, err := bl.runProc(args)
		if err != nil {
			log.Printf("%v: runProc %v err %v status %v\n", db.GetName(), args, err, status)
		}
		if err != nil && (strings.HasPrefix(err.Error(), "Spawn error") ||
			strings.HasPrefix(err.Error(), "EOF")) {
			log.Fatalf("%v: runProc err %v\n", db.GetName(), err)
		}
		if retryf(err, status) {
			// log.Printf("%v: proc %v err %v status %v\n", db.GetName(), args, err, status)
		} else {
			return nil
		}
	}
	return nil
}

// XXX run in parallel?
func (bl *Balancer) runMovers(nextShards []string) []string {
	moved := []string{}
	for i, kvd := range bl.conf.Shards {
		if kvd != nextShards[i] {
			shard := strconv.Itoa(i)
			s := shardPath(kvd, shard)
			d := shardPath(nextShards[i], shard)
			moved = append(moved, s)
			bl.runProcRetry([]string{"bin/user/kv-mover", s, d}, func(err error, status string) bool {
				return err != nil || status != "OK"
			})
		}
	}
	return moved
}

func (bl *Balancer) runDeleters(moved []string) {
	for _, sharddir := range moved {
		bl.runProcRetry([]string{"bin/user/kv-deleter", sharddir},
			func(err error, status string) bool {
				ok := strings.HasPrefix(status, "file not found")
				return err != nil || (status != "OK" && !ok)
			})
	}
}

func (bl *Balancer) balance(opcode, mfs string) error {
	var err error

	bl.conf, err = readConfig(bl.FsLib, KVCONFIG)
	if err != nil {
		log.Fatalf("%v: readConfig: err %v\n", db.GetName(), err)
	}

	log.Printf("%v: BAL Balancer: %v %v %v\n", db.GetName(), opcode, mfs, bl.conf)

	if bl.isBusy {
		return fmt.Errorf("retry")
	}

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

	bl.isBusy = true
	defer func() {
		bl.isBusy = false
	}()

	log.Printf("%v: BAL conf %v next shards: %v\n", db.GetName(), bl.conf, nextShards)

	err = bl.lease.RenameTo(KVCONFIGBK)
	if err != nil {
		log.Printf("%v: Rename to %v err %v\n", db.GetName(), KVCONFIGBK, err)
	}

	var moved []string
	if bl.conf.N == 0 {
		bl.initShards(nextShards)
	} else {
		moved = bl.runMovers(nextShards)
	}

	bl.conf.N += 1
	bl.conf.Shards = nextShards
	bl.conf.Moved = moved
	bl.conf.Ctime = time.Now().UnixNano()

	log.Printf("%v: new %v\n", db.GetName(), bl.conf)

	err = atomic.MakeFileJsonAtomic(bl.FsLib, KVNEXTCONFIG, 0777, *bl.conf)
	if err != nil {
		log.Printf("%v: MakeFile %v err %v\n", db.GetName(), KVNEXTCONFIG, err)
		// XXX maybe crash
	}

	err = bl.lease.MakeLeaseFileFrom(KVNEXTCONFIG)
	if err != nil {
		log.Printf("%v: rename %v -> %v: error %v\n", db.GetName(), KVNEXTCONFIG, KVCONFIG, err)
		// XXX maybe crash
	}

	bl.runDeleters(moved)

	err = bl.Remove(KVCONFIGBK)
	if err != nil {
		log.Printf("%v: Remove %v err %v\n", db.GetName(), KVCONFIGBK, err)
	}
	return nil
}
