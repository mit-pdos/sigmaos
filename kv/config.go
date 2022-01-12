package kv

import (
	"fmt"
	"log"
	"time"

	"ulambda/fslib"
)

type Move struct {
	Src string
	Dst string
}

type Moves []*Move

func (mvs Moves) String() string {
	s := "["
	for _, m := range mvs {
		if m != nil {
			s += fmt.Sprintf("%v -> %v", m.Src, m.Dst)
		}
	}
	s += "]"
	return s
}

type Config struct {
	N      int
	Shards []string // slice mapping shard # to server
	Moves  Moves    // shards to be deleted because they moved
	Ctime  int64    // XXX use ctime of config file?
}

func (cf *Config) String() string {
	return fmt.Sprintf("{N %v, Shards %v, Moves %v}", cf.N, cf.Shards, cf.Moves)
}

func MakeConfig(n int) *Config {
	cf := &Config{n, make([]string, NSHARD), Moves{}, 0}
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

func (ks *KvSet) high(notkv string) (string, int) {
	h := ""
	n := 0
	for k, v := range ks.set {
		if v > n && k != notkv {
			h = k
			n = v
		}
	}
	return h, n
}

func (ks *KvSet) low() (string, int) {
	l := ""
	n := NSHARD
	for k, v := range ks.set {
		if v < n && k != "" {
			l = k
			n = v
		}
	}
	return l, n
}

func (ks *KvSet) nshards(kv string) int {
	return ks.set[kv]
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

// assign to t shards from hkv to newkv
func assign(conf *Config, nextShards []string, hkv string, t int, newkv string) []string {
	for i, kv := range nextShards {
		if kv == hkv {
			nextShards[i] = newkv
			t -= 1
			if t <= 0 {
				break
			}
		}
	}
	return nextShards
}

func AddKv(conf *Config, newkv string) []string {
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
	// log.Printf("add: n = %v\n", n)
	for i := 0; i < n; {
		hkv, h := kvs.high(newkv)
		t := 1
		if h-n >= 1 {
			t = h - n
		}
		// log.Printf("n = %v h = %v t = %v %v->%v\n", n, h, t, hkv, newkv)
		nextShards = assign(conf, nextShards, hkv, t, newkv)
		kvs.set[hkv] -= t
		kvs.set[newkv] += t
		i += t
	}
	return nextShards
}

func DelKv(conf *Config, delkv string) []string {
	nextShards := make([]string, NSHARD)
	kvs := makeKvs(conf.Shards)
	n := kvs.nshards(delkv)
	kvs.del([]string{delkv})

	l := len(kvs.mkKvs())
	p := n / l
	n1 := (NSHARD + l - 1) / l
	// log.Printf("del: n = %v p = %v n1 = %v\n", n, p, n1)
	for i, kv := range conf.Shards {
		nextShards[i] = kv
	}
	for i := n; i > 0; {
		lkv, l := kvs.low()
		t := p
		if i < p {
			t = i

		}
		if l+t > n1 {
			t = 1
		}
		// log.Printf("i = %v l = %v t = %v %v->%v\n", i, l, t, delkv, lkv)
		nextShards = assign(conf, nextShards, delkv, t, lkv)
		kvs.set[lkv] += t
		i -= t
	}
	return nextShards
}
