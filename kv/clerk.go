package kv

import (
	"crypto/rand"
	"hash/fnv"
	"log"
	"math/big"
	"strconv"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
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
	fsl      *fslib.FsLib
	uname    string
	conf     Config
	nextConf Config
	ch       chan bool
	nget     int
}

func MakeClerk() *KvClerk {
	kc := &KvClerk{}
	kc.ch = make(chan bool)
	kc.uname = "clerk/" + strconv.FormatUint(nrand(), 16)
	db.Name(kc.uname)
	kc.fsl = fslib.MakeFsLib(kc.uname)
	err := kc.fsl.ReadFileJson(KVCONFIG, &kc.conf)
	if err != nil {
		// XXX deal with clerk starting during view change
		log.Fatalf("CLERK: readConfig %v error %v\n", KVCONFIG, err)
	}
	return kc
}

func (kc *KvClerk) watch(path string) {
	kc.ch <- true
}

// XXX atomic read
func (kc *KvClerk) readConfig() {
	// set watch for conf, which indicates commit to view change
	for {
		err := kc.fsl.ReadFileJsonWatch(KVCONFIG, &kc.nextConf, kc.watch)
		if err == nil {
			db.DLPrintf("KV", "readConfig: %v\n", kc.nextConf)
			if kc.nextConf.N == kc.conf.N+1 {
				kc.conf = kc.nextConf
				break
			}
			log.Fatalf("View mismatch %v %v", kc.conf.N, kc.nextConf.N)
		} else if strings.HasPrefix(err.Error(), "file not found") {
			// wait for config
			db.DLPrintf("CLERK", "Wait for config %v\n", kc.conf.N+1)
			<-kc.ch
		} else {
			log.Fatalf("CLERK: readConfig %v error %v\n", KVCONFIG, err)
		}
	}
}

func error2shard(error string) string {
	kv := ""
	if strings.HasPrefix(error, "file not found") {
		i := strings.LastIndex(error, " ")
		kv = error[i+1:]
	}
	return kv
}

func doRetry(err error) bool {
	shard := error2shard(err.Error())
	if err.Error() == "EOF" || err.Error() == "Version mismatch" ||
		strings.HasPrefix(shard, "shard") ||
		err.Error() == "Closed by server" {
		return true
	}
	return false
}

func (kc *KvClerk) Put(k, v string) error {
	shard := key2shard(k)
	for {
		n := keyPath(kc.conf.Shards[shard], shard, kc.conf.N, k)
		err := kc.fsl.MakeFile(n, 0777, []byte(v))
		if err == nil {
			return err
		}
		db.DLPrintf("CLERK", "Put: %v %v %v\n", n, err, shard)
		if doRetry(err) {
			kc.readConfig()
		} else {
			return err
		}
	}
}

func (kc *KvClerk) Get(k string) (string, error) {
	shard := key2shard(k)
	for {
		n := keyPath(kc.conf.Shards[shard], shard, kc.conf.N, k)
		b, err := kc.fsl.Get(n)
		db.DLPrintf("CLERK", "Get: %v %v\n", n, err)
		if err == nil {
			kc.nget += 1
			return string(b), err
		}
		if doRetry(err) {
			kc.readConfig()
		} else {
			return string(b), err
		}
	}
}
