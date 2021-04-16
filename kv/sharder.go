package kv

//
// Shard coordinator: assigns shards to KVs.  Assumes no KV failures
// This is a short-lived daemon: it rebalances shards and then exists.
//

import (
	"fmt"
	"log"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/memfsd"
	np "ulambda/ninep"
)

const (
	NSHARD          = 10
	KVDIR           = "name/kv"
	SHARDER         = KVDIR + "/sharder"
	KVCONFIG        = KVDIR + "/config"
	KVNEXTCONFIG    = KVDIR + "/nextconfig"
	KVNEXTCONFIGTMP = KVDIR + "/nextconfigtmp"
	KVCOMMIT        = KVDIR + "/commit/"
	KVLOCK          = KVDIR + "/lock"
)

func commitName(kv string) string {
	return KVCOMMIT + kv
}

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

type Sharder struct {
	*fslib.FsLibSrv
	ch       chan string
	pid      string
	args     []string
	kvs      []string // the kv servers in this configuration
	nextKvs  []string // the available kvs for the next config
	nkvd     int      // # KVs in reconfiguration
	conf     *Config
	nextConf *Config
	done     bool
}

func MakeSharder(args []string) (*Sharder, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("MakeSharder: too few arguments %v\n", args)
	}
	sh := &Sharder{}
	sh.ch = make(chan string)
	sh.pid = args[0]
	sh.args = args[1:]
	db.Name("sharder")
	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("MakeSharder: no IP %v\n", err)
	}
	fsd := memfsd.MakeFsd(ip + ":0")
	db.DLPrintf("SHARDER", "New sharder %v", args)
	fls, err := fslib.InitFs(SHARDER, fsd, nil)
	if err != nil {
		return nil, err
	}
	sh.FsLibSrv = fls
	sh.Started(sh.pid)
	return sh, nil
}

// Caller holds lock
// XXX minimize movement
func (sh *Sharder) balance() *Config {
	j := 0
	conf := makeConfig(sh.conf.N + 1)

	db.DLPrintf("SHARDER", "shards %v (len %v) kvs %v\n", sh.conf.Shards,
		len(sh.conf.Shards), sh.nextKvs)

	if len(sh.nextKvs) == 0 {
		return conf
	}
	for i, _ := range sh.conf.Shards {
		conf.Shards[i] = sh.nextKvs[j]
		j = (j + 1) % len(sh.nextKvs)
	}
	return conf
}

func (sh *Sharder) Exit() {
	sh.ExitFs(SHARDER)
}

func (sh *Sharder) readConfig(conffile string) *Config {
	conf := Config{}
	err := sh.ReadFileJson(conffile, &conf)
	if err != nil {
		return nil
	}
	sh.kvs = make([]string, 0)
	for _, kv := range conf.Shards {
		present := false
		if kv == "" {
			continue
		}
		for _, k := range sh.kvs {
			if k == kv {
				present = true
				break
			}
		}
		if !present {
			sh.kvs = append(sh.kvs, kv)
		}
	}
	return &conf
}

func (sh *Sharder) watchPrepared(p string) {
	db.DLPrintf("SHARDER", "watchPrepared %v\n", p)
	sh.ch <- p
}

func (sh *Sharder) makeNextConfig() {
	err := sh.MakeFileJson(KVNEXTCONFIGTMP, *sh.nextConf)
	if err != nil {
		return
	}
	err = sh.Rename(KVNEXTCONFIGTMP, KVNEXTCONFIG)
	if err != nil {
		db.DLPrintf("SHARDER", "SHARDER: rename %v -> %v: error %v\n",
			KVNEXTCONFIGTMP, KVNEXTCONFIG, err)
		return
	}
}

func (sh *Sharder) lock() {
	_, err := sh.CreateFile(KVLOCK, 0777|np.DMTMP, np.OWRITE|np.OCEXEC)
	if err != nil {
		log.Fatalf("Lock failed %v\n", err)
	}
}

func (sh *Sharder) unlock() {
	err := sh.Remove(KVLOCK)
	if err != nil {
		log.Fatalf("Unlock failed failed %v\n", err)
	}
}

func (sh *Sharder) Work() {
	// log.Printf("SHARDER %v Sharder: %v %v\n", sh.pid, sh.conf, sh.args)

	sh.lock()
	defer sh.unlock()

	sh.conf = sh.readConfig(KVCONFIG)

	db.DLPrintf("SHARDER", "Sharder: %v %v\n", sh.conf, sh.args)

	switch sh.args[0] {
	case "add":
		sh.nextKvs = append(sh.kvs, sh.args[1:]...)
		fn := commitName(sh.args[1])
		// set watch for existence of fn, which indicates is ready to prepare
		_, err := sh.ReadFileWatch(fn, sh.watchPrepared)
		if err == nil {
			db.DLPrintf("SHARDER", "KV %v started", fn)
		} else {
			db.DLPrintf("SHARDER", "Wait for %v", fn)
			<-sh.ch
		}
	case "del":
		sh.nextKvs = make([]string, len(sh.kvs))
		copy(sh.nextKvs, sh.kvs)
		for _, del := range sh.args[1:] {
			for i, kv := range sh.nextKvs {
				if del == kv {
					sh.nextKvs = append(sh.nextKvs[:i],
						sh.nextKvs[i+1:]...)
				}
			}
		}
	case "check":
		return
	default:
		log.Fatalf("Unknown command %v\n", sh.args[0])
	}

	sh.nextConf = sh.balance()
	db.DLPrintf("SHARDER", "Sharder next conf: %v %v\n", sh.nextConf, sh.nextKvs)

	sts, err := sh.ReadDir(KVCOMMIT)
	if err != nil {
		log.Fatalf("SHARDER: ReadDir commit error %v\n", err)
	}

	for _, st := range sts {
		fn := KVCOMMIT + st.Name
		err = sh.Remove(fn)
		if err != nil {
			db.DLPrintf("SHARDER", "Remove %v failed %v\n", fn, err)
		}
	}

	if sh.args[0] == "del" {
		sh.nextKvs = append(sh.nextKvs, sh.args[1:]...)

	}

	sh.nkvd = len(sh.nextKvs)
	for _, kv := range sh.nextKvs {
		fn := KVCOMMIT + kv
		// set watch for existence of fn, which indicates fn has prepared
		_, err := sh.ReadFileWatch(fn, sh.watchPrepared)
		if err == nil {
			log.Fatalf("SHARDER: watch failed %v", err)
		}
	}

	err = sh.Remove(KVCONFIG)
	if err != nil {
		db.DLPrintf("SHARDER", "Remove %v failed %v\n", KVCONFIG, err)
	}

	sh.makeNextConfig()

	for i := 0; i < sh.nkvd; i++ {
		s := <-sh.ch
		db.DLPrintf("SHARDER", "KV %v prepared\n", s)
	}

	db.DLPrintf("SHARDER", "Commit to %v\n", sh.nextConf)
	// commit to new config
	err = sh.Rename(KVNEXTCONFIG, KVCONFIG)
	if err != nil {
		db.DLPrintf("SHARDER", "SHARDER: rename %v -> %v: error %v\n",
			KVNEXTCONFIG, KVCONFIG, err)
		return
	}
}
