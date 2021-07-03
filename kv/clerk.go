package kv

import (
	"crypto/rand"
	"hash/fnv"
	"log"
	"math/big"
	"strconv"
	"strings"
	// "time"

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
	fsl   *fslib.FsLib
	uname string
	conf  Config
	ch    chan bool
	nget  int
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

func (kc *KvClerk) Exit() {
	kc.fsl.Exit()
}

// XXX atomic read
func (kc *KvClerk) readConfig() {
	for {
		err := kc.fsl.ReadFileJson(KVCONFIG, &kc.conf)
		if err == nil {
			break
		}
		err = kc.fsl.ReadFileJsonWatch(KVCONFIG, &kc.conf, kc.watchConfig)
		if err != nil {
			<-kc.ch
		} else {
			log.Fatalf("CLERK: Watch %v error %v\n", KVCONFIG, err)
		}
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
		fn := keyPath(kc.conf.Shards[shard], strconv.Itoa(shard), k)
		err := kc.fsl.MakeFile(fn, 0777, []byte(v))
		if err == nil {
			return err
		}
		db.DLPrintf("CLERK", "Put: %v %v %v\n", fn, err, shard)
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
		fn := keyPath(kc.conf.Shards[shard], strconv.Itoa(shard), k)
		b, err := kc.fsl.Get(fn)
		db.DLPrintf("CLERK", "Get: %v %v\n", fn, err)
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

func (kc *KvClerk) KVs() []string {
	kc.readConfig()
	kcs := makeKvs(kc.conf.Shards)
	return kcs.mkKvs()
}
