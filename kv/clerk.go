package kv

import (
	"crypto/rand"
	"log"
	"math/big"
	"strconv"
	"time"

	"ulambda/fslib"
)

func key2shard(key string) int {
	shard := 0
	if len(key) > 0 {
		shard = int(key[0])
	}
	shard %= NSHARDS
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

func (kc *KvClerk) Put(k, v string) error {
	shard := key2shard(k)
	for {
		kvd := kc.conf.Shards[shard]
		n := kvd + "/" + strconv.Itoa(kc.conf.N) + "-" + k
		err := kc.MakeFile(n, []byte(v))
		if err == nil {
			return err
		}
		log.Printf("Put: MakeFile: %v %v\n", n, err)
		if err.Error() == ErrWrongKv.Error() {
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
		kvd := kc.conf.Shards[shard]
		n := kvd + "/" + strconv.Itoa(kc.conf.N) + "-" + k
		b, err := kc.ReadFile(n)
		if err == nil {
			return string(b), err
		}
		log.Printf("Get: ReadFile: %v (s %v) %v\n", n, shard, err)
		if err.Error() == ErrWrongKv.Error() {
			kc.readConfig()
		} else if err.Error() == ErrRetry.Error() {
			time.Sleep(100 * time.Millisecond)
		} else {
			return string(b), err
		}
	}
}
