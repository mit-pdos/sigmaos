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
	"ulambda/memfs"
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
	kv.me = KV + "/" + kv.pid
	db.Name(kv.me)
	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("MakeKv: no IP %v\n", err)
	}
	fsd := memfsd.MakeFsd(ip+":0", func(uname string) npo.CtxI {
		return memfs.MkCtx(uname, kv)
	})
	fsl, err := fslib.InitFs(kv.me, fsd, &KvDev{kv})
	if err != nil {
		return nil, err
	}
	kv.FsLibSrv = fsl
	kv.Started(kv.pid)
	return kv, nil
}

func (kv *Kv) ParsePath(ctx *memfs.Ctx, path []string) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if len(path) == 0 { // so that ls in root directory works
		return nil
	}

	if strings.HasPrefix(ctx.Uname(), "clerk/") &&
		strings.Contains(path[len(path)-1], "-") {
		if kv.nextConf != nil {
			db.DLPrintf("KV", "ParsePath: %v %v retry\n", ctx, path)
			return ErrRetry
		} else {
			p := strings.Split(path[len(path)-1], "-")
			if p[0] != strconv.Itoa(kv.conf.N) {
				db.DLPrintf("KV", "ParsePath: %v %v redirect\n", ctx, path)
				return ErrWrongKv
			}
			shard := key2shard(p[1])
			if kv.conf.Shards[shard] != kv.pid {
				db.DLPrintf("KV", "ParsePath: %v %v redirect1\n", ctx, path)
				return ErrWrongKv
			}
			path[len(path)-1] = p[1]
			return nil
		}
	}
	return nil
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

func keyPath(kvd string, shard int, k string) string {
	d := shardPath(kvd, shard)
	return d + "/" + k
}

// make directories for new shards i should hold. cannot hold lock on
// kv, since Walk() must take it.
func (kv *Kv) makeShardDirs() {
	for s, kvd := range kv.nextConf.Shards {
		if kvd == kv.pid && kv.conf.Shards[s] != kv.pid {
			d := shardPath(kv.pid, s)
			err := kv.Mkdir(d, 07)
			if err != nil {
				log.Fatalf("%v: moveShards: mkdir %v err %v\n",
					kv.me, d, err)
			}
		}
	}
}

// copy new shards to me.
func (kv *Kv) moveShards() {
	for s, kvd := range kv.conf.Shards {
		if kvd != kv.pid && kv.nextConf.Shards[s] == kv.pid {
			src := shardPath(kvd, s)
			dst := shardPath(kv.pid, s)
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
		if kvd != kv.pid && kv.conf.Shards[s] == kv.pid {
			d := shardPath(kv.pid, s)
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

// Caller holds lock
func (kv *Kv) prepare() {
	kv.conf = kv.readConfig(KVCONFIG)
	kv.nextConf = kv.readConfig(KVNEXTCONFIG)

	db.DLPrintf("KV", "prepare for new config: %v %v\n", kv.conf, kv.nextConf)

	kv.mu.Unlock()
	defer kv.mu.Lock()

	kv.makeShardDirs()
	if kv.nextConf.N > 1 {
		kv.moveShards()
	}
	kv.prepared()
}

// Caller holds lock
func (kv *Kv) commit() bool {
	db.DLPrintf("KV", "commit to new config: %v\n", kv.nextConf)

	kv.removeShards()

	kv.conf = kv.nextConf
	kv.nextConf = nil

	for _, kvd := range kv.conf.Shards {
		if kvd == kv.pid {
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
