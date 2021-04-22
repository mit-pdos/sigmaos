package kv

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	//"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

const (
	KV = "name/kv"
)

func kvname(pid string) string {
	return "kv" + pid
}

type Kv struct {
	mu sync.Mutex
	*fslib.FsLibSrv
	done     chan bool
	pid      string
	me       string
	args     []string
	conf     *Config
	nextConf *Config
}

func MakeKv(args []string) (*Kv, error) {
	kv := &Kv{}
	kv.done = make(chan bool)
	if len(args) != 2 {
		return nil, fmt.Errorf("MakeKv: too few arguments %v\n", args)
	}
	kv.pid = args[0]
	kv.args = args
	kv.me = kvname(kv.pid)
	db.Name(kv.me)
	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("MakeKv: no IP %v\n", err)
	}
	fsd := memfsd.MakeFsd(ip + ":0")
	fsl, err := fslib.InitFs(KV+"/"+kv.me, fsd, nil)
	if err != nil {
		return nil, err
	}
	kv.FsLibSrv = fsl
	kv.Started(kv.pid)

	kv.conf, err = kv.readConfig(KVCONFIG)
	if err != nil {
		log.Fatalf("KV: MakeKv cannot read %v err %v\n", KVCONFIG, err)
	}

	// set watch for existence, indicates view change
	_, err = kv.readConfigWatch(KVNEXTCONFIG, kv.watchNextConf)
	if err != nil {
		db.DLPrintf("KV", "MakeKv set watch on %v (err %v)\n", KVNEXTCONFIG, err)
	}

	db.DLPrintf("KV", "Spawn harder\n")

	pid1 := kv.spawnSharder(args[1], kv.me)
	ok, err := kv.Wait(pid1)

	db.DLPrintf("KV", "Sharder done %v\n", string(ok))

	// XXX fix once Wait returns appropriate exit status
	if args[1] == "crash1" {
		log.Printf("KV: sharder crashed\n")
		return nil, fmt.Errorf("Wait/Sharder failed %v %v\n", err, string(ok))
	}

	if err != nil || string(ok) != "OK" {
		return nil, fmt.Errorf("Wait/Sharder failed %v %v\n", err, string(ok))
	}
	return kv, nil
}

func (kv *Kv) spawnSharder(opcode, pid string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/sharderd"
	a.Args = []string{opcode, pid}
	a.PairDep = nil
	a.ExitDep = nil
	kv.Spawn(&a)
	return a.Pid
}

func (kv *Kv) watchNextConf(p string, err error) {
	db.DLPrintf("KV", "Watch fires %v %v; prepare?\n", p, err)
	if err == nil {
		kv.prepare()
	} else {
		_, err = kv.readConfigWatch(KVNEXTCONFIG, kv.watchNextConf)
		if err == nil {
			db.DLPrintf("KV", "watchNextConf: next conf %v (err %v)\n", KVNEXTCONFIG, err)
			kv.prepare()
		} else {
			db.DLPrintf("KV", "Commit: set watch on %v (err %v)\n", KVNEXTCONFIG, err)
		}
	}
}

func (kv *Kv) readConfig(conffile string) (*Config, error) {
	conf := Config{}
	err := kv.ReadFileJson(conffile, &conf)
	return &conf, err
}

func (kv *Kv) readConfigWatch(conffile string, f fsclnt.Watch) (*Config, error) {
	conf := Config{}
	err := kv.ReadFileJsonWatch(conffile, &conf, f)
	return &conf, err
}

func shardPath(kvd string, shard, view int) string {
	return KVDIR + "/" + kvd + "/shard" + strconv.Itoa(shard) + "-v" + strconv.Itoa(view)
}

func keyPath(kvd string, shard int, view int, k string) string {
	d := shardPath(kvd, shard, view)
	return d + "/" + k
}

func shardTmp(shardp string) string {
	return shardp + "#"
}

// Move shard: either copy to new shard server or rename shard dir
// for new view.
func (kv *Kv) moveShard(s int, kvd string) error {
	src := shardPath(kv.me, s, kv.conf.N)
	src = shardTmp(src)
	if kvd != kv.me { // Copy
		dst := shardPath(kvd, s, kv.nextConf.N)
		err := kv.Mkdir(dst, 0777)
		// an aborted view change may have created the directory
		if err != nil && !strings.HasPrefix(err.Error(), "Name exists") {
			return err
		}
		db.DLPrintf("KV", "Copy shard from %v to %v\n", src, dst)
		err = kv.CopyDir(src, dst)
		if err != nil {
			return err
		}
		db.DLPrintf("KV", "Copy shard from %v to %v done\n", src, dst)
	} else { // rename
		dst := shardPath(kvd, s, kv.nextConf.N)
		err := kv.Rename(src, dst)
		if err != nil {
			log.Printf("KV Rename %v -> %v failed %v\n", src, dst, err)
		}
	}
	return nil
}

func (kv *Kv) moveShards() error {
	if kv.conf == nil {
		panic("KV kc.conf")
	}
	if kv.nextConf == nil {
		panic("KV next conf")
	}
	for s, kvd := range kv.conf.Shards {
		if kvd == kv.me && kv.nextConf.Shards[s] != "" {
			if err := kv.moveShard(s, kv.nextConf.Shards[s]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (kv *Kv) restoreShards() {
	for s, kvd := range kv.conf.Shards {
		if kvd == kv.me {
			src := shardPath(kv.me, s, kv.conf.N)
			src = shardTmp(src)
			dst := shardPath(kv.me, s, kv.conf.N)
			err := kv.Rename(src, dst)
			if err != nil {
				log.Printf("KV Restore %v -> %v failed %v\n", src, dst, err)
				src := shardPath(kvd, s, kv.nextConf.N)
				err := kv.Rename(src, dst)
				if err != nil {
					log.Printf("KV Restore %v -> %v failed %v\n", src, dst, err)
				}
			}

		}
	}
}

// Make intial shard directories
func (kv *Kv) initShards() {
	if kv.nextConf == nil {
		panic("next conf")
	}
	for s, kvd := range kv.nextConf.Shards {
		dst := shardPath(kvd, s, kv.nextConf.N)
		db.DLPrintf("KV", "Init shard dir %v\n", dst)
		err := kv.Mkdir(dst, 0777)
		if err != nil {
			log.Fatalf("KV %v: initShards: mkdir %v err %v\n", kv.me, dst, err)
		}
	}
}

func (kv *Kv) removeShards() {
	for s, kvd := range kv.nextConf.Shards {
		if kvd != kv.me && kv.conf.Shards[s] == kv.me {
			d := shardPath(kv.me, s, kv.conf.N)
			d = shardTmp(d)
			db.DLPrintf("KV", "RmDir shard %v\n", d)
			err := kv.RmDir(d)
			if err != nil {
				log.Fatalf("KV %v: moveShards: remove %v err %v\n",
					kv.me, d, err)
			}
		}
	}
}

// Tell sharder we are prepared to commit new config
func (kv *Kv) prepared(status string) {
	fn := prepareName(kv.me)
	db.DLPrintf("KV", "Prepared %v\n", fn)
	err := kv.MakeFileAtomic(fn, 0777, []byte(status))
	if err != nil {
		db.DLPrintf("KV", "Prepared: make file %v failed %v\n", fn, err)
	}
}

func (kv *Kv) committed() {
	fn := commitName(kv.me)
	db.DLPrintf("KV", "Committed %v\n", fn)
	err := kv.MakeFile(fn, 0777, []byte("OK"))
	if err != nil {
		db.DLPrintf("KV", "Committed: make file %v failed %v\n", fn, err)
	}
}

func (kv *Kv) unpostShard(s, old int) {
	fn := shardPath(kv.me, s, old)
	db.DLPrintf("KV", "unpostShard: %v %v\n", fn, kv.Addr())
	err := kv.Rename(fn, shardTmp(fn))
	if err != nil {
		log.Printf("KV %v Remove failed %v\n", kv.me, err)
	}
}

func (kv *Kv) unpostShards() {
	for i, kvd := range kv.conf.Shards {
		if kvd == kv.me {
			kv.unpostShard(i, kv.conf.N)
		}
	}
}

func (kv *Kv) closeFid(shard string) {
	db.DLPrintf("KV", "closeFids shard %v\n", shard)
	kv.ConnTable().IterateFids(func(f *npo.Fid) {
		p1 := np.Join(f.Path())
		uname := f.Ctx().Uname()
		if strings.HasPrefix(uname, "clerk") && strings.HasPrefix(p1, shard) {
			db.DLPrintf("KV", "CloseFid: mark closed %v %v\n", uname, p1)
			f.Close()
		}
	})
}

// Close fids for which i will not be responsible, signaling to
// clients to failover to another server.
func (kv *Kv) closeFids() {
	for s, kvd := range kv.nextConf.Shards {
		if kvd != kv.me && kv.conf.Shards[s] == kv.me {
			kv.closeFid("shard" + strconv.Itoa(s))
		}
	}
}

func (kv *Kv) watchConf(p string, err error) {
	db.DLPrintf("KV", "Watch conf fires %v %v; commit\n", p, err)
	kv.commit()
}

// XXX maybe check if one is already running?
func (kv *Kv) restartSharder() {
	log.Printf("KV %v watchSharder: SHARDER crashed\n", kv.me)
	pid1 := kv.spawnSharder("restart", kv.me)
	ok, err := kv.Wait(pid1)
	if err != nil {
		log.Printf("KV wait failed\n")
	}
	log.Printf("KV Sharder done %v\n", string(ok))

}

func (kv *Kv) watchSharder(p string, err error) {
	kv.mu.Lock()
	done := kv.nextConf == nil
	kv.mu.Unlock()

	log.Printf("KV Watch sharder fires %v %v done? %v\n", p, err, done)

	// sharder may have exited because it is done. if I am not in
	// 2PC, then assume sharder exited because it is done.
	// clerks are able to do puts/gets.
	if done {
		return
	}

	if err == nil {
		kv.restartSharder()
	} else {
		// set remove watch on sharder in case it crashes during 2PC
		err = kv.SetRemoveWatch(SHARDER, kv.watchSharder)
		if err != nil {
			kv.restartSharder()
		}
	}
}

func (kv *Kv) prepare() {
	kv.mu.Lock()

	var err error

	log.Printf("KV %v prepare\n", kv.me)

	// set remove watch on sharder in case it crashes during 2PC
	err = kv.SetRemoveWatch(SHARDER, kv.watchSharder)
	if err != nil {
		db.DLPrintf("KV", "Prepare: SHARDER crashed\n")
	}

	// set watch for when old config file is replaced (indicates commit)
	err = kv.SetRemoveWatch(KVCONFIG, kv.watchConf)
	if err != nil {
		log.Fatalf("KV %v: SetRemoveWatch %v err %v\n", kv.me, KVCONFIG, err)
	}
	db.DLPrintf("KV", "prepare: watch for %v\n", KVCONFIG)
	kv.nextConf, err = kv.readConfig(KVNEXTCONFIG)
	if err != nil {
		log.Fatalf("KV %v: KV prepare cannot read %v err %v\n", kv.me, KVNEXTCONFIG, err)
	}

	db.DLPrintf("KV", "prepare for new config: %v %v\n", kv.conf, kv.nextConf)

	if kv.nextConf.N != kv.conf.N+1 {
		log.Fatalf("KV %v: Skipping to %d from %d", kv.me, kv.nextConf.N, kv.conf.N)
	}

	kv.unpostShards()

	kv.closeFids()

	kv.mu.Unlock()

	if kv.nextConf.N > 1 {
		if err := kv.moveShards(); err != nil {
			log.Printf("%v: moveShards %v\n", kv.me, err)
			kv.prepared("ABORT")
			return
		}
	} else {
		kv.initShards()
	}

	if kv.args[1] == "crash4" {
		db.DLPrintf("KV", "Crashed in prepare\n")
		os.Exit(1)
	}

	kv.prepared("OK")
}

func (kv *Kv) commit() {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	log.Printf("KV %v commit/abort\n", kv.me)

	conf, err := kv.readConfig(KVCONFIG)
	if err != nil {
		log.Fatalf("KV commit cannot read %v err %v\n", KVCONFIG, err)
	}

	if conf.N == kv.nextConf.N {
		log.Printf("%v: KV commit to next config %v\n", kv.me, kv.nextConf)
		kv.removeShards()
	} else {
		log.Printf("%v: KV abort to next config %v\n", kv.me, conf)
		kv.restoreShards()
		kv.nextConf = conf
	}

	if kv.args[1] == "crash5" {
		db.DLPrintf("KV", "Crashed in commit/abort\n")
		os.Exit(1)
	}

	present := kv.nextConf.present(kv.me)

	kv.conf = kv.nextConf
	kv.nextConf = nil

	if present {
		// reset watch for existence of nextconfig, which indicates view change
		_, err = kv.readConfigWatch(KVNEXTCONFIG, kv.watchNextConf)
		if err == nil {
			db.DLPrintf("KV", "Commit to next conf %v (err %v)\n", KVNEXTCONFIG, err)
			go func() {
				kv.prepare()
			}()
		} else {
			db.DLPrintf("KV", "Commit: set watch on %v (err %v)\n", KVNEXTCONFIG, err)
		}
	}

	kv.committed()

	if !present {
		db.DLPrintf("KV", "commit exit %v\n", kv.me)
		kv.done <- true
		return
	}
}

func (kv *Kv) Work() {
	db.DLPrintf("KV", "Work\n")
	<-kv.done
	db.DLPrintf("KV", "exit %v\n", kv.conf)
}

func (kv *Kv) Exit() {
	kv.ExitFs(kv.me)
}
