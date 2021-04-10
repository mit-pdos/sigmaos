package kv

import (
	"crypto/rand"
	"hash/fnv"
	"log"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

func key2shard(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := int(h.Sum32() % NSHARD)
	return shard
}

func nrand() uint64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Uint64()
	return x
}

type KvClerk struct {
	mu       sync.Mutex
	fsl      *fslib.FsLib
	uname    string
	conf     Config
	nextConf Config
}

func MakeClerk() (*KvClerk, error) {
	kc := &KvClerk{}
	kc.uname = "clerk/" + strconv.FormatUint(nrand(), 16)
	db.Name(kc.uname)
	kc.fsl = fslib.MakeFsLib(kc.uname)
	err := kc.readConfig(&kc.conf)
	return kc, err
}

func shardSrvName(s int) string {
	return "shardSrv" + strconv.Itoa(s)
}

func (kc *KvClerk) watch(path string) {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	db.DLPrintf("CLERK", "watch: Config changed %v\n", path)
	err := kc.readConfig(&kc.nextConf)
	if err != nil {
		log.Printf("watch: readConfig error %v\n", err)
	}
	db.DLPrintf("CLERK", "watch: cur conf %v new conf %v\n", kc.conf, kc.nextConf)
	for i, s := range kc.nextConf.Shards {
		if kc.conf.Shards[i] != s {
			p := KVDIR + "/" + shardSrvName(i)
			p1 := np.Split(p)
			db.DLPrintf("CLERK", "Umount %v\n", p1)
			err := kc.fsl.Umount(p1)
			if err != nil {
				log.Printf("CLERK: umount %v failed %v\n", p1, err)
			}
		}
	}
	kc.conf.N = kc.nextConf.N
	copy(kc.conf.Shards, kc.nextConf.Shards)
}

func (kc *KvClerk) readConfig(conf *Config) error {
	err := kc.fsl.ReadFileJsonWatch(KVCONFIG, conf, kc.watch)
	if err != nil {
		log.Printf("readConfig error %v\n", err)
	}
	db.DLPrintf("CLERK", "readConfig: conf %v\n", conf)
	return err
}

func (kc *KvClerk) keyPath(shard int, k string) string {
	return KVDIR + "/" + shardSrvName(shard) + "/shard" + strconv.Itoa(shard) + "/" + k
}

func error2shard(error string) string {
	kv := ""
	if strings.HasPrefix(error, "file not found") {
		i := strings.LastIndex(error, " ")
		kv = error[i+1:]
	}
	return kv
}

func (kc *KvClerk) Put(k, v string) error {
	shard := key2shard(k)
	for {
		n := kc.keyPath(shard, k)
		err := kc.fsl.MakeFile(n, []byte(v))
		if err == nil {
			return err
		}
		shard := error2shard(err.Error())
		db.DLPrintf("CLERK", "Put: %v %v %v\n", n, err, shard)
		if err.Error() == "EOF" || err.Error() == "Version mismatch" ||
			strings.HasPrefix(shard, "shardSrv") ||
			err.Error() == "Closed by server" {
			time.Sleep(100 * time.Millisecond)
		} else {
			return err
		}
	}
}

func (kc *KvClerk) Get(k string) (string, error) {
	shard := key2shard(k)
	for {
		n := kc.keyPath(shard, k)
		b, err := kc.fsl.Get(n)
		db.DLPrintf("CLERK", "Get: %v %v\n", n, err)
		if err == nil {
			return string(b), err
		}
		shard := error2shard(err.Error())
		if err.Error() == "EOF" || err.Error() == "Version mismatch" ||
			strings.HasPrefix(shard, "shardSrv") ||
			err.Error() == "Closed by server" {
			time.Sleep(100 * time.Millisecond)
		} else {
			return string(b), err
		}
	}
}
