package kv

import (
	"fmt"
	"log"
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
	conf     *Config
	nextConf *Config
}

func MakeKv(args []string) (*Kv, error) {
	kv := &Kv{}
	kv.done = make(chan bool)
	if len(args) != 1 {
		return nil, fmt.Errorf("MakeKv: too few arguments %v\n", args)
	}
	kv.pid = args[0]
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
		log.Fatalf("MakeKv cannot read %v err %v\n", KVCONFIG, err)
	}
	// set watch for existence, indicates view change
	_, err = kv.readConfigWatch(KVNEXTCONFIG, kv.watchNextConf)
	if err != nil {
		db.DLPrintf("KV", "MakeKv set watch on %v (err %v)\n", KVNEXTCONFIG, err)
	}
	kv.prepared()
	return kv, nil
}

func (kv *Kv) watchNextConf(p string) {
	db.DLPrintf("KV", "Watch fires %v; prepare\n", p)

	kv.prepare()
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
func (kv *Kv) moveShard(s int, kvd string) {
	src := shardPath(kv.me, s, kv.conf.N)
	src = shardTmp(src)
	if kvd != kv.me { // Copy
		dst := shardPath(kvd, s, kv.nextConf.N)
		err := kv.Mkdir(dst, 0777)
		if err != nil {
			log.Fatalf("%v: makeShardDirs: mkdir %v err %v\n",
				kv.me, dst, err)
		}
		db.DLPrintf("KV", "Copy shard from %v to %v\n", src, dst)
		err = kv.CopyDir(src, dst)
		if err != nil {
			log.Fatalf("KV copyDir: %v %v err %v\n", src, dst, err)
		}
		db.DLPrintf("KV", "Copy shard from %v to %v done\n", src, dst)
	} else { // rename
		dst := shardPath(kvd, s, kv.nextConf.N)
		err := kv.Rename(src, dst)
		if err != nil {
			log.Printf("KV Rename failed %v\n", err)
		}
	}

}

func (kv *Kv) moveShards() {
	if kv.conf == nil {
		panic("KV kc.conf")
	}
	if kv.nextConf == nil {
		panic("KV next conf")
	}
	for s, kvd := range kv.conf.Shards {
		if kvd == kv.me && kv.nextConf.Shards[s] != "" {
			kv.moveShard(s, kv.nextConf.Shards[s])
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
			log.Fatalf("%v: initShards: mkdir %v err %v\n", kv.me, dst, err)
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
				log.Fatalf("%v: moveShards: remove %v err %v\n",
					kv.me, d, err)
			}
		}
	}
}

// Tell sharder we are prepared to commit new config
// XXX make this file ephemeral
func (kv *Kv) prepared() {
	fn := commitName(kv.me)
	db.DLPrintf("KV", "Prepared %v\n", fn)
	err := kv.MakeFile(fn, 0777|np.DMTMP, nil)
	if err != nil {
		db.DLPrintf("KV", "Prepared: make file %v failed %v\n", fn, err)
	}
}

func (kv *Kv) unpostShard(s, old int) {
	fn := shardPath(kv.me, s, old)
	db.DLPrintf("KV", "unpostShard: %v %v\n", fn, kv.Addr())
	err := kv.Rename(fn, shardTmp(fn))
	if err != nil {
		log.Printf("Remove failed %v\n", err)
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

func (kv *Kv) watchConf(p string) {
	db.DLPrintf("KV", "Watch fires %v; commit\n", p)
	kv.commit()
}

func (kv *Kv) prepare() {
	kv.mu.Lock()

	var err error

	// set watch for new config file (indicates commit)
	_, err = kv.readConfigWatch(KVCONFIG, kv.watchConf)
	if err == nil {
		log.Fatalf("KV prepare can read %v err %v\n", KVCONFIG, err)
	}
	kv.nextConf, err = kv.readConfig(KVNEXTCONFIG)
	if err != nil {
		log.Fatalf("KV prepare cannot read %v err %v\n", KVNEXTCONFIG, err)
	}

	db.DLPrintf("KV", "prepare for new config: %v %v\n", kv.conf, kv.nextConf)

	if kv.nextConf.N != kv.conf.N+1 {
		log.Fatalf("KV Skipping to %d from %d", kv.nextConf.N, kv.conf.N)
	}

	kv.unpostShards()

	kv.closeFids()

	kv.mu.Unlock()

	if kv.nextConf.N > 1 {
		kv.moveShards()
	} else {
		kv.initShards()
	}
	kv.prepared()
}

func (kv *Kv) watchKV(path string) {
	p := np.Split(path)
	kvd := p[len(p)-1]
	log.Printf("KV watch fired %v act? %v\n", kvd, kv.conf.present(kvd))
}

// If new, set watch on all KVs, except me. Otherwise, set watch on
// new ones (i have already watch on the ones in conf).
func (kv *Kv) watchKVs() {
	done := make(map[string]bool)
	old := kv.conf.present(kv.me)
	for _, kvd := range kv.nextConf.Shards {
		if kvd == "" {
			continue
		}
		if kvd == kv.me {
			continue
		}
		if old && kv.conf.present(kvd) {
			continue
		}
		// set watch if haven't set yet
		_, ok := done[kvd]
		if !ok {
			done[kvd] = true
			fn := KVDIR + "/" + kvd
			db.DLPrintf("KV", "Set watch on %v\n", fn)
			err := kv.RemoveWatch(fn, kv.watchKV)
			if err != nil {
				log.Fatalf("Remove watch err %v\n", fn)
			}
		}
	}
}

func (kv *Kv) commit() {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	db.DLPrintf("KV", "commit to new config: %v\n", kv.nextConf)

	kv.removeShards()

	kv.watchKVs()

	kv.conf = kv.nextConf
	kv.nextConf = nil

	// reset watch for existence of nextconfig, which indicates view change
	_, err := kv.readConfigWatch(KVNEXTCONFIG, kv.watchNextConf)
	if err != nil {
		db.DLPrintf("KV", "Commit: set watch on %v (err %v)\n", KVNEXTCONFIG, err)
	}

	for _, kvd := range kv.conf.Shards {
		if kvd == kv.me {
			return
		}
	}

	db.DLPrintf("KV", "commit exit %v\n", kv.me)
	kv.done <- true
}

func (kv *Kv) Work() {
	db.DLPrintf("KV", "Work\n")
	<-kv.done
	db.DLPrintf("KV", "exit %v\n", kv.conf)
}

func (kv *Kv) Exit() {
	kv.ExitFs(kv.me)
}
