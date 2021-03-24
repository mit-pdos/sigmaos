package kv

import (
	"crypto/rand"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"ulambda/fslib"
)

func key2shard(key string) int {
	shard := 0
	if len(key) > 0 {
		shard = int(key[0])
	}
	shard %= NSHARD
	return shard
}

func nrand() uint64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Uint64()
	return x
}

type KvClerk struct {
	*fslib.FsLib
	conf  Config
	uname string
}

func MakeClerk() (*KvClerk, error) {
	kc := &KvClerk{}
	kc.uname = "clerk/" + strconv.FormatUint(nrand(), 16)
	kc.FsLib = fslib.MakeFsLib(kc.uname)
	err := kc.readConfig()
	return kc, err
}

func (kc *KvClerk) readConfig() error {
	err := kc.ReadFileJson(KVCONFIG, &kc.conf)
	if err != nil {
		log.Printf("readConfig error %v\n", err)
	}
	log.Printf("%v: read config %v\n", kc.uname, kc.conf)
	return err
}

func (kc *KvClerk) keyPath(shard int, k string) string {
	kvd := kc.conf.Shards[shard]
	return shardPath(kvd, shard) + "/" + strconv.Itoa(kc.conf.N) + "-" + k

}

func error2kv(error string) string {
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
		err := kc.MakeFile(n, []byte(v))
		if err == nil {
			return err
		}
		// log.Printf("%v: Put: MakeFile: %v %v\n", kc.uname, n, err)
		kv := error2kv(err.Error())
		if err.Error() == ErrWrongKv.Error() || err.Error() == "EOF" ||
			kv == kc.conf.Shards[shard] {
			kc.readConfig()
		} else if err.Error() == ErrRetry.Error() {
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
		b, err := kc.ReadFile(n)
		if err == nil {
			return string(b), err
		}
		kv := error2kv(err.Error())
		// XXX als check for Fid version mismatch
		if err.Error() == ErrWrongKv.Error() || err.Error() == "EOF" ||
			kv == kc.conf.Shards[shard] {
			kc.readConfig()
		} else if err.Error() == ErrRetry.Error() {
			time.Sleep(100 * time.Millisecond)
		} else {
			return string(b), err
		}
	}
}
