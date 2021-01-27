package kv

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
)

const (
	KV      = "name/kv"
	SHARDER = "name/kv/sharder"
)

type KvDev struct {
	kv *Kv
}

func (kvdev *KvDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	log.Printf("KvDev.write %v\n", t)
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
	log.Printf("Kv: %v\n", args)
	kv.pid = args[0]
	kv.me = KV + "/" + kv.pid

	fs := memfs.MakeRoot()
	fsd := memfsd.MakeFsd(fs, kv)
	fsl, err := fslib.InitFsMemFsD(kv.me, fs, fsd, &KvDev{kv})
	if err != nil {
		return nil, err
	}
	kv.FsLibSrv = fsl
	kv.Started(kv.pid)
	kv.conf = kv.readConfig(KVCONFIG)
	return kv, nil
}

func (kv *Kv) join() error {
	sh := SHARDER + "/dev"
	log.Printf("Join %v\n", kv.me)
	err := kv.WriteFile(sh, []byte("Join "+kv.me))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", sh, err)
	}
	return err
}

// Interposes on memfsd's walk to check that clerk and I run in same config
func (kv *Kv) Walk(src string, names []string) error {
	db.DPrintf("%v: Walk %v %v\n", kv.me, src, names)
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if len(names) == 0 { // so that ls in root directory works
		return nil
	}
	if names[0] == "dev" {
		return nil
	}
	if strings.HasPrefix(src, "clerk/") &&
		strings.Contains(names[len(names)-1], "-") {
		if kv.nextConf != nil {
			return ErrRetry
		} else {
			p := strings.Split(names[len(names)-1], "-")
			if p[0] != strconv.Itoa(kv.conf.N) {
				return ErrWrongKv
			}
			shard := key2shard(p[1])
			if kv.conf.Shards[shard] != kv.me {
				return ErrWrongKv
			}
			names[len(names)-1] = p[1]
			return nil
		}
	}
	return nil
}

func (kv *Kv) readConfig(conffile string) *Config {
	conf := Config{}
	err := kv.ReadFileJson(conffile, &conf)
	if err != nil {
		log.Fatalf("ReadFileJson: %v\n", err)
	}
	return &conf
}

func shardPath(kvd string, shard int) string {
	return kvd + "/" + strconv.Itoa(shard)
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
				log.Fatalf("%v: moveShards: mkdir %v err %v\n",
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
			err := kv.Remove(d)
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
	log.Printf("%v: prepared %v\n", kv.me, sh)
	err := kv.WriteFile(sh, []byte("Prepared "+kv.me))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", sh, err)
	}
}

// Caller hold lock
func (kv *Kv) prepare() {
	kv.nextConf = kv.readConfig(KVNEXTCONFIG)

	log.Printf("%v: prepare for new config: %v\n", kv.me, kv.nextConf)

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
	log.Printf("%v: commit to new config: %v\n", kv.me, kv.nextConf)

	kv.removeShards()

	kv.conf = kv.nextConf
	kv.nextConf = nil

	for _, kvd := range kv.conf.Shards {
		if kvd == kv.me {
			return true
		}
	}

	log.Printf("%v: exit\n", kv.me)
	return false
}

func (kv *Kv) Work() {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.join()
	cont := true
	for cont {
		kv.cond.Wait()
		if kv.nextConf == nil {
			kv.prepare()
		} else {
			cont = kv.commit()
		}
	}
}

func (kv *Kv) Exit() {
	kv.ExitFs(kv.me)
	kv.Exiting(kv.pid)
}
