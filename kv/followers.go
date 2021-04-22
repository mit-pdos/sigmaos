package kv

import (
	"fmt"
	"log"

	"ulambda/fsclnt"
	"ulambda/fslib"
)

type Followers struct {
	*fslib.FsLib
	kvs map[string]bool
}

func mkFollowers(fsl *fslib.FsLib, shards []string) *Followers {
	fw := &Followers{}
	fw.FsLib = fsl
	fw.kvs = make(map[string]bool)
	for _, kv := range shards {
		if _, ok := fw.kvs[kv]; !ok && kv != "" {
			fw.kvs[kv] = true
		}
	}
	return fw
}

func (fw *Followers) String() string {
	return fmt.Sprintf("%v", fw.mkKvs())
}

func (fw *Followers) len() int {
	return len(fw.kvs)
}

func (fw *Followers) add(new []string) {
	for _, kv := range new {
		fw.kvs[kv] = true
	}
}

func (fw *Followers) del(old []string) {
	for _, kv := range old {
		delete(fw.kvs, kv)
	}
}

func (fw *Followers) mkKvs() []string {
	kvs := make([]string, 0, len(fw.kvs))
	for kv, _ := range fw.kvs {
		kvs = append(kvs, kv)
	}
	return kvs
}

func (fw *Followers) setStatusWatches(dir string, f fsclnt.Watch) {
	for kv, _ := range fw.kvs {
		fn := dir + kv
		// set watch for existence of fn, which indicates fn
		// has prepared/committed
		_, err := fw.ReadFileWatch(fn, f)
		if err == nil {
			log.Fatalf("SHARDER: set status watch failed %v", err)
		}
	}
}

func (fw *Followers) setKVWatches(f fsclnt.Watch) {
	for kv, _ := range fw.kvs {
		// set watch for KV, in case it crashes during 2PC
		err := fw.SetRemoveWatch(KVDIR+"/"+kv, f)
		if err != nil {
			log.Fatalf("SHARDER: set KV watch failed %v", err)
		}
	}
}
