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
	confNext Config
	ch       chan bool
	nget     int
}

func MakeClerk() *KvClerk {
	kc := &KvClerk{}
	kc.ch = make(chan bool)
	kc.uname = "clerk/" + strconv.FormatUint(nrand(), 16)
	db.Name(kc.uname)
	kc.fsl = fslib.MakeFsLib(kc.uname)
	kc.readConfig()
	return kc
}

func (kc *KvClerk) watchConfig(path string, err error) {
	db.DLPrintf("CLERK", "watch fired %v\n", path)
	kc.ch <- true
}

func (kc *KvClerk) watchNext(path string, err error) {
	db.DLPrintf("CLERK", "watch next fired %v\n", path)
	kc.readConfig()
}

// set watch for conf, which indicates commit to view change
// XXX atomic read
func (kc *KvClerk) readConfig() {
	err := kc.fsl.ReadFileJson(KVCONFIG, &kc.conf)
	if err != nil {
		log.Fatalf("CLERK: ReadFileJson %v error %v\n", KVCONFIG, err)

	}
	err = kc.fsl.SetRemoveWatch(KVCONFIG, kc.watchConfig)
	if err != nil {
		log.Fatalf("CLERK: SetRemoveWatch %v error %v\n", KVCONFIG, err)
	}
	err = kc.fsl.ReadFileJsonWatch(KVNEXTCONFIG, &kc.confNext, kc.watchNext)
	if err == nil {
		<-kc.ch
	} else if err != nil && !strings.HasPrefix(err.Error(), "file not found") {
		log.Fatalf("CLERK: ReadFileJsonWatch %v error %v\n", KVNEXTCONFIG, err)
	}
	db.DLPrintf("CLERK", "readConfig %v\n", kc.conf)
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
