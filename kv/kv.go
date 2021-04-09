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

type KvDev struct {
	kv *Kv
}

func kvname(pid string) string {
	return "kv" + pid
}

func (kvdev *KvDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	if strings.HasPrefix(t, "Prepare") {
		kvdev.kv.cond.Signal()
	} else if strings.HasPrefix(t, "Commit") {
		kvdev.kv.cond.Signal()
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}

	return np.Tsize(len(data)), nil
}

func (kvdev *KvDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	//	if off == 0 {
	//	s := kvdev.sd.ps()
	//return []byte(s), nil
	//}
	return nil, nil
}

func (kvdev *KvDev) Len() np.Tlength {
	return 0
}

type Kv struct {
	mu   sync.Mutex
	cond *sync.Cond
	*fslib.FsLibSrv
	pid      string
	me       string
	conf     *Config
	nextConf *Config
}

func MakeKv(args []string) (*Kv, error) {
	kv := &Kv{}
	kv.cond = sync.NewCond(&kv.mu)
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
	fsl, err := fslib.InitFs(KV+"/"+kv.me, fsd, &KvDev{kv})
	if err != nil {
		return nil, err
	}
	kv.FsLibSrv = fsl
	kv.Started(kv.pid)
	return kv, nil
}

func (kv *Kv) readConfig(conffile string) *Config {
	conf := Config{}
	err := kv.ReadFileJson(conffile, &conf)
	if err != nil {
		return nil
	}
	return &conf
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
	kv.mu.Unlock()
	defer kv.mu.Lock()

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
	sh := SHARDER + "/dev"
	db.DLPrintf("KV", "prepared %v\n", sh)
	err := kv.WriteFile(sh, []byte("Prepared "+kv.Addr()))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", sh, err)
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
		db.DLPrintf("Symlink %v failed %v\n", fn, err)
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

func (kv *Kv) closeFids() {
	for s, kvd := range kv.nextConf.Shards {
		if kvd != kv.me && kv.conf.Shards[s] == kv.me {
			kv.closeFid("shard" + strconv.Itoa(s))
		}
	}
}

// Caller holds lock
func (kv *Kv) prepare() {
	kv.conf = kv.readConfig(KVCONFIG)
	kv.nextConf = kv.readConfig(KVNEXTCONFIG)

	db.DLPrintf("KV", "prepare for new config: %v %v\n", kv.conf, kv.nextConf)

	kv.unpostShards()

	kv.closeFids()

	kv.mu.Unlock()
	defer kv.mu.Lock()

	kv.makeShardDirs()
	if kv.nextConf.N > 1 {
		kv.moveShards()
	}
	kv.postShards()
	kv.prepared()
}

// Caller holds lock
func (kv *Kv) commit() bool {
	db.DLPrintf("KV", "commit to new config: %v\n", kv.nextConf)

	kv.removeShards()

	kv.conf = kv.nextConf
	kv.nextConf = nil

	for _, kvd := range kv.conf.Shards {
		if kvd == kv.me {
			return true
		}
	}

	db.DLPrintf("KV", "commit exit %v\n", kv.me)
	return false
}

func (kv *Kv) Work() {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	db.DLPrintf("KV", "Work\n")
	cont := true
	for cont {
		kv.cond.Wait()
		if kv.nextConf == nil {
			kv.prepare()
		} else {
			cont = kv.commit()
		}
	}
	// log.Printf("%v: exit %v\n", kv.me, kv.conf)
}

func (kv *Kv) Exit() {
	kv.ExitFs(kv.me)
	kv.Exiting(kv.pid, "OK")
}
