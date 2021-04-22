package kv

import (
	db "ulambda/debug"
	"ulambda/fslib"
)

type Config struct {
	N      int
	Shards []string // maps shard # to server
}

func makeConfig(n int) *Config {
	cf := &Config{n, make([]string, NSHARD)}
	return cf
}

func (cf *Config) present(n string) bool {
	for _, s := range cf.Shards {
		if s == n {
			return true
		}
	}
	return false
}

func readConfig(fsl *fslib.FsLibSrv, conffile string) *Config {
	conf := Config{}
	err := fsl.ReadFileJson(conffile, &conf)
	if err != nil {
		return nil
	}
	return &conf
}

// XXX minimize movement
func balance(conf *Config, nextFw *Followers) *Config {
	j := 0
	new := makeConfig(conf.N + 1)

	db.DLPrintf("SHARDER", "balance %v (len %v) kvs %v\n", conf.Shards,
		len(conf.Shards), nextFw)

	kvs := nextFw.mkKvs()
	if len(kvs) == 0 {
		return new
	}

	for i, _ := range conf.Shards {
		new.Shards[i] = kvs[j]
		j = (j + 1) % len(kvs)
	}
	return new
}
