package kv

//
// Shard balancer.
//

import (
	"fmt"
	"log"
	"strconv"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
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
	pid      string
	args     []string
	conf     *Config
	nextConf *Config2
}

func MakeBalancer(args []string) (*Balancer, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("MakeBalancer: too few arguments %v\n", args)
	}
	bl := &Balancer{}
	bl.pid = args[0]
	bl.args = args[1:]
	bl.FsLib = fslib.MakeFsLib(bl.pid)

	db.Name("balancer")

	if err := bl.LockFile(KVDIR, KVLOCK); err != nil {
		log.Fatalf("Lock failed %v\n", err)
	}

	bl.Started(bl.pid)
	return bl, nil
}

func (bl *Balancer) unlock() {
	if err := bl.UnlockFile(KVDIR, KVLOCK); err != nil {
		log.Fatalf("Unlock failed failed %v\n", err)
	}
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
func (bl *Balancer) unpostShards() {
	for i, kvd := range bl.nextConf.Old {
		if kvd != bl.nextConf.New[i] {
			bl.unpostShard(kvd, strconv.Itoa(i))
		}
	}
}

// Make intial shard directories
func (bl *Balancer) initShards() {
	for s, kvd := range bl.nextConf.New {
		dst := shardPath(kvd, strconv.Itoa(s))
		db.DLPrintf("BAL", "Init shard dir %v\n", dst)
		err := bl.Mkdir(dst, 0777)
		if err != nil {
			log.Fatalf("BAL mkdir %v err %v\n", dst, err)
		}
	}
}

func (bl *Balancer) spawnMover(s, src, dst string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/mover"
	a.Args = []string{s, src, dst}
	a.PairDep = nil
	a.ExitDep = nil
	bl.Spawn(&a)
	return a.Pid
}

func (bl *Balancer) runMovers(ks *KvSet) {
	for i, kvd := range bl.nextConf.Old {
		if kvd != bl.nextConf.New[i] {
			pid1 := bl.spawnMover(strconv.Itoa(i), kvd, bl.nextConf.New[i])
			ok, err := bl.Wait(pid1)
			if string(ok) != "OK" || err != nil {
				log.Printf("mover %v failed %v err %v\n", kvd,
					string(ok), err)
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

	kvs := makeKvs(bl.conf.Shards)

	switch bl.args[0] {
	case "add":
		kvs.add(bl.args[1:])
	case "del":
		kvs.del(bl.args[1:])
	default:
	}

	bl.nextConf = balance(bl.conf, kvs)

	db.DLPrintf("BAL", "Balancer conf %v next conf: %v %v\n", bl.conf,
		bl.nextConf, kvs)

	// log.Printf("BAL conf %v next conf: %v %v\n", bl.conf, bl.nextConf, kvs)

	err = bl.Rename(KVCONFIG, KVCONFIGBK)
	if err != nil {
		db.DLPrintf("BAL", "BAL: Rename to %v err %v\n", KVCONFIGBK, err)
	}

	if bl.nextConf.N > 1 {
		bl.unpostShards()
	}

	if bl.nextConf.N == 1 {
		bl.initShards()
	} else {
		bl.runMovers(kvs)
	}

	bl.conf.N = bl.nextConf.N
	bl.conf.Shards = bl.nextConf.New
	bl.conf.Ctime = time.Now().UnixNano()

	log.Printf("new %v\n", bl.conf)

	err = bl.MakeFileJsonAtomic(KVNEXTCONFIG, 0777, *bl.conf)
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
