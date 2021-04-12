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
		db.DLPrintf("KV", "MakeKv cannot read %v err %v\n", KVCONFIG, err)
	}
	_, err = kv.readConfigWatch(KVNEXTCONFIG, kv.watchNextConf)
	if err != nil {
		db.DLPrintf("KV", "MakeKv set watch on %v (err %v)\n", KVNEXTCONFIG, err)
	}
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

func shardPath(kvd string, shard int) string {
	return KVDIR + "/" + kvd + "/shard" + strconv.Itoa(shard)
}

func shardPath1(shard int) string {
	return KVDIR + "/shardSrv" + strconv.Itoa(shard)
}

func keyPath(kvd string, shard int, k string) string {
	d := shardPath(kvd, shard)
	return d + "/" + k
}

// make directories for new shards i should hold. cannot hold lock on
// kv, since Walk() must take it.
func (kv *Kv) makeShardDirs() {
	for s, kvd := range kv.nextConf.Shards {
		if kvd == kv.me && kv.conf.Shards[s] != kv.me {
			d := shardPath(kv.me, s)
			err := kv.Mkdir(d, 0777)
			if err != nil {
				log.Fatalf("%v: makeShardDirs: mkdir %v err %v\n",
					kv.me, d, err)
			}
		}
	}
}

// copy new shards to me.
func (kv *Kv) moveShards() {
	for s, kvd := range kv.conf.Shards {
		if kvd != kv.me && kv.nextConf.Shards[s] == kv.me {
			src := shardPath(kvd, s)
			dst := shardPath(kv.me, s)
			db.DLPrintf("KV", "Copy shard from %v to %v\n", src, dst)
			err := kv.CopyDir(src, dst)
			if err != nil {
				log.Fatalf("copyDir: %v %v err %v\n", src, dst, err)
			}
		}
	}
}

func (kv *Kv) removeShards() {
	for s, kvd := range kv.nextConf.Shards {
		if kvd != kv.me && kv.conf.Shards[s] == kv.me {
			d := shardPath(kv.me, s)
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
func (kv *Kv) prepared() {
	fn := KVCOMMIT + kv.me
	err := kv.MakeFile(fn, nil)
	if err != nil {
		log.Printf("WriteFile: %v %v\n", fn, err)
	}
}

func (kv *Kv) unpostShard(i int) {
	fn := shardPath1(i)
	db.DLPrintf("KV", "unpostShard: %v %v\n", fn, kv.Addr())
	err := kv.Remove(fn)
	if err != nil {
		log.Printf("Remove failed %v\n", err)
	}
}

// XXX unpost only the ones i am not responsible for anymore
func (kv *Kv) unpostShards() {
	for i, kvd := range kv.conf.Shards {
		if kvd == kv.me {
			kv.unpostShard(i)
		}
	}
}

func (kv *Kv) postShard(i int) {
	fn := shardPath1(i)
	db.DLPrintf("KV", "postShard: %v %v\n", fn, kv.Addr())
	err := kv.Symlink(kv.Addr()+":pubkey", fn, 0777|np.DMTMP)
	if err != nil {
		db.DLPrintf("KV", "Symlink %v failed %v\n", fn, err)
		panic("postShard")
	}
}

func (kv *Kv) postShards() {
	for i, kvd := range kv.nextConf.Shards {
		if kvd == kv.me {
			kv.postShard(i)
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
	kv.conf, err = kv.readConfigWatch(KVCONFIG, kv.watchConf)
	if err != nil {
		log.Fatalf("prepare cannot read %v err %v\n", KVCONFIG, err)
	}
	kv.nextConf, err = kv.readConfig(KVNEXTCONFIG)
	if err != nil {
		log.Fatalf("prepare cannot read %v err %v\n", KVNEXTCONFIG, err)
	}

	db.DLPrintf("KV", "prepare for new config: %v %v\n", kv.conf, kv.nextConf)

	kv.unpostShards()

	kv.closeFids()

	kv.mu.Unlock()

	kv.makeShardDirs()

	if kv.nextConf.N > 1 {
		kv.moveShards()
	}
	kv.prepared()
}

func (kv *Kv) commit() {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	db.DLPrintf("KV", "commit to new config: %v\n", kv.nextConf)

	kv.postShards()

	kv.removeShards()

	kv.conf = kv.nextConf
	kv.nextConf = nil
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
