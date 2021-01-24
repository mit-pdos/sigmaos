package kv

import (
	"log"

	"ulambda/fslib"
)

type KvClerk struct {
	*fslib.FsLib
	conf Config
}

func MakeClerk() (*KvClerk, error) {
	kc := &KvClerk{}
	kc.FsLib = fslib.MakeFsLib(false)
	err := kc.readConfig()
	return kc, err
}

func (kc *KvClerk) readConfig() error {
	err := kc.ReadFileJson(KVCONFIG, &kc.conf)
	if err != nil {
		log.Printf("readConfig error %v\n", err)
	}
	return err
}

func key2shard(key string) int {
	shard := 0
	if len(key) > 0 {
		shard = int(key[0])
	}
	shard %= NSHARDS
	return shard
}

func (kc *KvClerk) Put(k, v string) error {
	shard := key2shard(k)
	for {
		kvd := kc.conf.Shards[shard]
		err := kc.MakeFile(kvd+"/"+k, []byte(v))
		if err != nil {
			log.Printf("Put: MakeFile: %v %v\n", k, err)
		}
		if err == ErrWrongKv {
			kc.readConfig()
		} else {
			return err
		}
	}
}

func (kc *KvClerk) Get(k string) (string, error) {
	shard := key2shard(k)
	for {
		kvd := kc.conf.Shards[shard]
		b, err := kc.ReadFile(kvd + "/" + k)
		if err != nil {
			log.Printf("Put: WriteFile: %v %v\n", k, err)
		}
		if err == ErrWrongKv {
			kc.readConfig()
		} else {
			return string(b), err
		}
	}
}
