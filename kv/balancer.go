package kv

//
// Shard balancer.
//

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"ulambda/atomic"
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procdep"
	"ulambda/procinit"
	"ulambda/sync"
)

const (
	NSHARD       = 10
	KVDIR        = "name/kv"
	KVCONFIG     = KVDIR + "/config"
	KVCONFIGBK   = KVDIR + "/config#"
	KVNEXTCONFIG = KVDIR + "/nextconfig"
	KVLOCK       = "lock"
)

type Balancer struct {
	*fslib.FsLib
	proc.ProcClnt
	pid    string
	args   []string
	conf   *Config
	kvlock *sync.Lock
}

func MakeBalancer(args []string) (*Balancer, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("MakeBalancer: too few arguments %v\n", args)
	}
	bl := &Balancer{}
	bl.pid = args[0]
	bl.args = args[1:]
	bl.FsLib = fslib.MakeFsLib(bl.pid)
	bl.ProcClnt = procinit.MakeProcClnt(bl.FsLib, procinit.GetProcLayersMap())
	bl.kvlock = sync.MakeLock(bl.FsLib, KVDIR, KVLOCK, true)

	db.Name("balancer")

	bl.kvlock.Lock()

	bl.Started(bl.pid)
	return bl, nil
}

func (bl *Balancer) unlock() {
	bl.kvlock.Unlock()
}

func (bl *Balancer) unpostShard(kv, s string) {
	fn := shardPath(kv, s)
	// db.DLPrintf("BAL", "unpostShard: %v\n", fn)
	err := bl.Rename(fn, shardTmp(fn))
	if err != nil {
		log.Printf("BAL %v Rename failed %v\n", fn, err)
	}
}

// Unpost shards that are moving
func (bl *Balancer) unpostShards(nextShards []string) {
	for i, kvd := range bl.conf.Shards {
		if kvd != nextShards[i] {
			bl.unpostShard(kvd, strconv.Itoa(i))
		}
	}
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
	t := procdep.MakeProcDep(proc.GenPid(), "bin/user/mover", []string{s, src, dst})
	t.Env = []string{procinit.GetProcLayersString()}
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

func (bl *Balancer) Balance() {
	var err error

	defer bl.unlock() // release lock acquired in MakeBalancer()

	// db.DLPrintf("BAL", "Balancer: %v\n", bl.args)

	bl.conf, err = readConfig(bl.FsLib, KVCONFIG)
	if err != nil {
		log.Fatalf("readConfig: err %v\n", err)
	}

	log.Printf("BAL Balancer: %v %v\n", bl.args, bl.conf)

	var nextShards []string
	switch bl.args[0] {
	case "add":
		// XXX call balanceAdd repeatedly for each bl.args[1:]
		nextShards = balanceAdd(bl.conf, bl.args[1])
	case "del":
		// XXX call balanceDel repeatedly for each bl.args[1:]
		nextShards = balanceDel(bl.conf, bl.args[1])
	default:
	}

	db.DLPrintf("BAL", "Balancer conf %v next shards: %v \n", bl.conf, nextShards)

	log.Printf("BAL conf %v next shards: %v\n", bl.conf, nextShards)

	err = bl.Rename(KVCONFIG, KVCONFIGBK)
	if err != nil {
		db.DLPrintf("BAL", "BAL: Rename to %v err %v\n", KVCONFIGBK, err)
	}

	if bl.conf.N > 0 {
		bl.unpostShards(nextShards)
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

	err = bl.Rename(KVNEXTCONFIG, KVCONFIG)
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

func (bl *Balancer) Exit() {
	bl.Exited(bl.pid, "OK")
}
