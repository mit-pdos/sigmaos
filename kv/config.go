package kv

import (
	"fmt"
	"log"
	"time"

	"ulambda/fslib"
)

type Config struct {
	N      int
	Shards []string // slice mapping shard # to server
	Moved  []string // shards to be deleted because they moved
	Ctime  int64    // XXX use ctime of config file?
}

func (cf *Config) String() string {
	return fmt.Sprintf("{N %v, Shards %v, Moved %v}", cf.N, cf.Shards, cf.Moved)
}

func MakeConfig(n int) *Config {
	cf := &Config{n, make([]string, NSHARD), []string{}, 0}
	return cf
}

func (cf *Config) Present(n string) bool {
	for _, s := range cf.Shards {
		if s == n {
			return true
		}
	}
	return false
}

func readConfig(fsl *fslib.FsLib, conffile string) (*Config, error) {
	conf := Config{}
	err := fsl.ReadFileJson(conffile, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

type KvSet struct {
	set map[string]int
}

func makeKvs(shards []string) *KvSet {
	ks := &KvSet{}
	ks.set = make(map[string]int)
	for _, kv := range shards {
		if _, ok := ks.set[kv]; !ok && kv != "" {
			ks.set[kv] = 0
		}
		if kv != "" {
			ks.set[kv] += 1
		}
	}
	return ks
}

func (ks *KvSet) mkKvs() []string {
	kvs := make([]string, 0, len(ks.set))
	for kv, _ := range ks.set {
		kvs = append(kvs, kv)
	}
	return kvs
}

func (ks *KvSet) add(new []string) {
	for _, kv := range new {
		ks.set[kv] = 0
	}
}

func (ks *KvSet) del(old []string) {
	for _, kv := range old {
		delete(ks.set, kv)
	}
}

func (ks *KvSet) first() string {
	memfs := ""
	for k := range ks.set {
		memfs = k
		break
	}
	return memfs
}

func (ks *KvSet) high() (string, int) {
	h := ""
	n := 0
	for k := range ks.set {
		if ks.set[k] > n {
			h = k
			n = ks.set[k]
		}
	}
	return h, n
}

func (ks *KvSet) low() (string, int) {
	l := ""
	n := NSHARD
	for k := range ks.set {
		if ks.set[k] < n && k != "" {
			l = k
			n = ks.set[k]
		}
	}
	return l, n
}

func readKVs(fsl *fslib.FsLib) *KvSet {
	for true {
		conf, err := readConfig(fsl, KVCONFIG)
		if err != nil {
			// balancer may be at work
			log.Printf("readKVs: err %v\n", err)
			time.Sleep(1000 * time.Millisecond)
			continue
		}
		kvs := makeKvs(conf.Shards)
		log.Printf("Monitor config %v\n", kvs)
		return kvs
	}
	return nil
}

func assign(conf *Config, nextShards []string, hkv string, n, t int, newkv string) {
	m := 0
	for i, kv := range conf.Shards {
		if kv == hkv {
			if m < n {
				nextShards[i] = hkv
				m += 1
			} else {
				nextShards[i] = newkv
				t -= 1
				if t <= 0 {
					break
				}
			}
		}
	}
}

func balanceAdd(conf *Config, newkv string) []string {
	nextShards := make([]string, NSHARD)
	kvs := makeKvs(conf.Shards)
	l := len(kvs.mkKvs()) + 1

	if l == 1 { // newkv is first shard
		for i, _ := range conf.Shards {
			nextShards[i] = newkv
		}
		return nextShards
	}

	for i, kv := range conf.Shards {
		nextShards[i] = kv
	}
	kvs.set[newkv] = 0
	n := (NSHARD + l - 1) / l

	for {
		hkv, h := kvs.high()
		if h-n <= 0 {
			break
		}
		t := h - n
		assign(conf, nextShards, hkv, n, t, newkv)
		kvs.set[hkv] -= t
		kvs.set[newkv] += t
	}

	// give newkv at least one shard
	if kvs.set[newkv] == 0 {
		hkv, h := kvs.high()
		if h > 1 {
			assign(conf, nextShards, hkv, n-1, 1, newkv)
		}
	}

	return nextShards
}

func balanceDel(conf *Config, delkv string) []string {
	nextShards := make([]string, NSHARD)
	kvs := makeKvs(conf.Shards)
	kvs.del([]string{delkv})
	l := len(kvs.mkKvs())
	n := (NSHARD + l - 1) / l
	for i, kv := range conf.Shards {
		nextShards[i] = kv
	}
	for {
		lkv, l := kvs.low()
		if n-l <= 0 {
			break
		}
		t1 := n - l
		for i, kv := range nextShards {
			nextShards[i] = kv
			if kv == delkv && t1 > 0 {
				nextShards[i] = lkv
				t1 -= 1
			}
		}
		kvs.set[lkv] += n - l
	}
	return nextShards
}
