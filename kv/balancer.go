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
	"ulambda/inode"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/sync"
)

const (
	NSHARD        = 10
	KVDIR         = "name/kv"
	KVCONFIG      = KVDIR + "/config"
	KVCONFIGBK    = KVDIR + "/config#"
	KVNEXTCONFIG  = KVDIR + "/nextconfig"
	KVLEASE       = KVDIR + "/lease"
	KVBALANCER    = KVDIR + "/balancer"
	KVBALANCERCTL = KVDIR + "/balancer/ctl"
)

type Balancer struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	conf     *Config
	ballease *sync.LeasePath
	lease    *sync.LeasePath
	mo       *Monitor
	ch       chan bool
}

func RunBalancer(auto, docrash string) {
	log.Printf("run balancer %v %v %v\n", proc.GetPid(), auto, docrash)

	bl := &Balancer{}
	bl.FsLib = fslib.MakeFsLib(proc.GetPid())
	bl.ProcClnt = procclnt.MakeProcClnt(bl.FsLib)

	// may fail if already exist
	bl.Mkdir(named.MEMFS, 07)
	bl.Mkdir(KVDIR, 07)
	bl.MakeFileJson(KVCONFIG, 0777, *MakeConfig(0))

	bl.ballease = sync.MakeLeasePath(bl.FsLib, KVLEASE)
	bl.lease = sync.MakeLeasePath(bl.FsLib, KVCONFIG)

	db.Name("balancer")

	// start server but don't publish
	mfs, _, err := fslibsrv.MakeMemFs("", "balancer")
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	err = dir.MkNod(fssrv.MkCtx(""), mfs.Root(), "ctl", makeCtl("balancer", mfs.Root(), bl))
	if err != nil {
		log.Fatalf("MakeNod clone failed %v\n", err)
	}

	ch := make(chan bool)
	go func() {
		mfs.Serve()
		ch <- true
	}()

	bl.ballease.WaitWLease()

	log.Printf("%v: primary\n", db.GetName())

	select {
	case <-ch:
		// done
	default:
		bl.recover()

		if docrash == "YES" {
			crash.Crasher(bl.FsLib)
		}

		// we are primary, post the balancer
		mfs.Post(KVBALANCER)

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

	log.Printf("balancer exited %v\n", proc.GetPid())
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

func (c *Ctl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	words := strings.Fields(string(b))
	if len(words) != 2 {
		return 0, fmt.Errorf("Invalid arguments")
	}
	c.bl.balance(words[0], words[1])
	return np.Tsize(len(b)), nil
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

func (bl *Balancer) recover() {
	var err error
	bl.conf, err = readConfig(bl.FsLib, KVCONFIG)
	if err == nil {
		log.Printf("recovery: nothing to do %v\n", bl.conf)
		return
	}
	err = bl.lease.MakeLeaseFileFrom(KVCONFIGBK)
	if err != nil {
		db.DLPrintf("BAL", "BAL: Rename from %v err %v\n", KVCONFIGBK, err)
	}
	log.Printf("recovery: restored config form %v\n", KVCONFIGBK)
}

// Make intial shard directories
func (bl *Balancer) initShards(nextShards []string) {
	for s, kvd := range nextShards {
		dst := shardPath(kvd, strconv.Itoa(s))
		db.DLPrintf("BAL", "Init shard dir %v\n", dst)
		err := bl.Mkdir(dst, 0777)
		if err != nil {
			log.Fatalf("BAL mkdir %v err %v\n", dst, err)
		}
	}
}

func (bl *Balancer) spawnMover(s, src, dst string) string {
	t := proc.MakeProc("bin/user/mover", []string{s, src, dst})
	bl.Spawn(t)
	return t.Pid
}

func (bl *Balancer) runMovers(nextShards []string) {
	for i, kvd := range bl.conf.Shards {
		if kvd != nextShards[i] {
			pid1 := bl.spawnMover(strconv.Itoa(i), kvd, nextShards[i])
			status, err := bl.WaitExit(pid1)
			if err != nil || status != "OK" {
				log.Printf("mover %v failed err %v status %v\n", kvd,
					err, status)
			}
		}
	}
}

func (bl *Balancer) balance(opcode, mfs string) {
	var err error

	// db.DLPrintf("BAL", "Balancer: %v\n", bl.args)

	bl.conf, err = readConfig(bl.FsLib, KVCONFIG)
	if err != nil {
		log.Fatalf("%v: readConfig: err %v\n", db.GetName(), err)
	}

	log.Printf("BAL Balancer: %v %v %v\n", opcode, mfs, bl.conf)

	var nextShards []string
	switch opcode {
	case "add":
		// XXX call balanceAdd repeatedly for each bl.args[1:]
		nextShards = balanceAdd(bl.conf, mfs)
	case "del":
		// XXX call balanceDel repeatedly for each bl.args[1:]
		nextShards = balanceDel(bl.conf, mfs)
	default:
	}

	db.DLPrintf("BAL", "Balancer conf %v next shards: %v \n", bl.conf, nextShards)

	log.Printf("BAL conf %v next shards: %v\n", bl.conf, nextShards)

	err = bl.lease.RenameTo(KVCONFIGBK)
	if err != nil {
		db.DLPrintf("BAL", "BAL: Rename to %v err %v\n", KVCONFIGBK, err)
	}

	if bl.conf.N == 0 {
		bl.initShards(nextShards)
	} else {
		bl.runMovers(nextShards)
	}

	bl.conf.N += 1
	bl.conf.Shards = nextShards
	bl.conf.Ctime = time.Now().UnixNano()

	log.Printf("new %v\n", bl.conf)

	err = atomic.MakeFileJsonAtomic(bl.FsLib, KVNEXTCONFIG, 0777, *bl.conf)
	if err != nil {
		db.DLPrintf("BAL", "BAL: MakeFile %v err %v\n", KVNEXTCONFIG, err)
	}

	err = bl.lease.MakeLeaseFileFrom(KVNEXTCONFIG)
	if err != nil {
		db.DLPrintf("BAL", "BAL: rename %v -> %v: error %v\n",
			KVNEXTCONFIG, KVCONFIG, err)
		return
	}
	err = bl.Remove(KVCONFIGBK)
	if err != nil {
		db.DLPrintf("BAL", "BAL: Remove %v err %v\n", KVCONFIGBK, err)
	}
}
