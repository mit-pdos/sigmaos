package kv

//
// Shard coordinator: assigns shards to KVs.  Assumes no KV failures
// This is a short-lived daemon: it rebalances shards and then exists.
//

import (
	"fmt"
	"log"
	"os"

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
	KVCONFIGTMP     = KVDIR + "/config#"
	KVNEXTCONFIG    = KVDIR + "/nextconfig"
	KVNEXTCONFIGTMP = KVDIR + "/nextconfig#"
	KVPREPARED      = KVDIR + "/prepared/"
	KVCOMMITTED     = KVDIR + "/committed/"
	KVLOCK          = KVDIR + "/lock"
)

func prepareName(kv string) string {
	return KVPREPARED + kv
}

func commitName(kv string) string {
	return KVCOMMITTED + kv
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

	// Grab KVLOCK before starting sharder
	fsl := fslib.MakeFsLib(SHARDER)
	_, err := fsl.CreateFile(KVLOCK, 0777|np.DMTMP, np.OWRITE|np.OCEXEC)
	if err != nil {
		log.Fatalf("Lock failed %v\n", err)
	}

	log.Printf("sharder: lock\n")

	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("MakeSharder: no IP %v\n", err)
	}
	fsd := memfsd.MakeFsd(ip + ":0")
	db.DLPrintf("SHARDER", "New sharder %v", args)
	fls, err := fslib.InitFsFsl(SHARDER, fsl, fsd, nil)
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

func (sh *Sharder) watchKV(p string, err error) {
	db.DLPrintf("SHARDER", "watchKV %v\n", p)
	sh.ch <- p
}

func (sh *Sharder) makeNextConfig() {
	err := sh.MakeFileJson(KVNEXTCONFIGTMP, 0777, *sh.nextConf)
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

func (sh *Sharder) unlock() {
	log.Printf("sharder unlock\n")
	err := sh.Remove(KVLOCK)
	if err != nil {
		log.Fatalf("Unlock failed failed %v\n", err)
	}
}

func (sh *Sharder) readStatus(dir string) map[string]bool {
	sts, err := sh.ReadDir(dir)
	if err != nil {
		return nil
	}
	kvs := make(map[string]bool)
	for _, st := range sts {
		kvs[st.Name] = true
	}
	return kvs
}

func (sh *Sharder) isCommitted(committed map[string]bool) bool {
	for _, kv := range sh.conf.Shards {
		if _, ok := committed[kv]; !ok {
			return false
		}
	}
	return true
}

func (sh *Sharder) doCommit(nextConf *Config, committed map[string]bool) bool {
	if nextConf == nil {
		return false
	}
	kvds := make(map[string]bool)
	for _, kv := range nextConf.Shards {
		if _, ok := kvds[kv]; !ok {
			kvds[kv] = true
		}
	}
	if committed == nil || len(committed) != len(kvds) {
		return false
	}
	for kv, _ := range kvds {
		if _, ok := committed[kv]; !ok {
			return false
		}
	}
	return true
}

func (sh *Sharder) abort() {
	db.DLPrintf("SHARDER", "Abort to %v\n", sh.conf)
	err := sh.Remove(KVNEXTCONFIG)
	if err != nil {
		db.DLPrintf("SHARDER", "Abort: remove %v failed %v\n", KVNEXTCONFIG, err)
	}
	err = sh.Rename(KVCONFIGTMP, KVCONFIG)
	if err != nil {
		db.DLPrintf("SHARDER", "Abort: rename %v failed %v\n", KVCONFIGTMP, err)
	}
}

func (sh *Sharder) restart() {
	sh.conf = sh.readConfig(KVCONFIG)
	sh.nextConf = sh.readConfig(KVNEXTCONFIG)
	prepared := sh.readStatus(KVPREPARED)
	committed := sh.readStatus(KVCOMMITTED)

	db.DLPrintf("SHARDER", "Restart: conf %v next %v prepared %v commit %v\n",
		sh.conf, sh.nextConf, prepared, committed)

	if sh.isCommitted(committed) {
		db.DLPrintf("SHARDER", "Restart: clean\n")
		return
	}

	if sh.doCommit(sh.nextConf, prepared) {
		db.DLPrintf("SHARDER", "Restart: commit\n")
		// XXX maybe subtract KVs that aren't live
		sh.commit(len(committed))
		// we committed to NEXTCONFIG; reread sh.conf to
		// get new this config
		sh.conf = sh.readConfig(KVCONFIG)
	} else {
		db.DLPrintf("SHARDER", "Restart: abort\n")
		sh.abort()
	}
}

func (sh *Sharder) Add() {
	sh.nextKvs = append(sh.kvs, sh.args[1:]...)
}

func (sh *Sharder) Del() {
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
}

func (sh *Sharder) rmKVFiles(dir string) {
	sts, err := sh.ReadDir(dir)
	if err != nil {
		log.Fatalf("SHARDER: ReadDir commit error %v\n", err)
	}
	for _, st := range sts {
		fn := dir + st.Name
		err = sh.Remove(fn)
		if err != nil {
			db.DLPrintf("SHARDER", "Remove %v failed %v\n", fn, err)
		}
	}
}

func (sh *Sharder) setKVWatches(dir string) {
	for _, kv := range sh.nextKvs {
		fn := dir + kv
		// set watch for existence of fn, which indicates fn
		// has prepared/committed
		_, err := sh.ReadFileWatch(fn, sh.watchKV)
		if err == nil {
			log.Fatalf("SHARDER: watch failed %v", err)
		}
	}
}

func (sh *Sharder) prepare() {

	sh.rmKVFiles(KVPREPARED)
	sh.rmKVFiles(KVCOMMITTED)

	sh.setKVWatches(KVPREPARED)

	sh.makeNextConfig()

	// depending how many KVs ack, crash2 results
	// in a abort or commit
	if sh.args[0] == "crash2" {
		db.DLPrintf("SHARDER", "Crash2\n")
		os.Exit(1)
	}

	for i := 0; i < sh.nkvd; i++ {
		s := <-sh.ch
		db.DLPrintf("SHARDER", "KV %v prepared\n", s)
	}
}

func (sh *Sharder) commit(ncommitted int) {
	db.DLPrintf("SHARDER", "Commit to %v\n", sh.nextConf)

	sh.setKVWatches(KVCOMMITTED)

	err := sh.Rename(KVNEXTCONFIG, KVCONFIG)
	if err != nil {
		db.DLPrintf("SHARDER", "SHARDER: rename %v -> %v: error %v\n",
			KVNEXTCONFIG, KVCONFIG, err)
		return
	}

	// crash3 should results in commit
	if sh.args[0] == "crash3" {
		db.DLPrintf("SHARDER", "Crash3\n")
		os.Exit(1)
	}

	for i := 0; i < sh.nkvd-ncommitted; i++ {
		s := <-sh.ch
		db.DLPrintf("SHARDER", "KV %v committed\n", s)
	}

	db.DLPrintf("SHARDER", "Done commit to %v\n", sh.nextConf)
}

func (sh *Sharder) TwoPC() {
	defer sh.unlock() // release lock acquired in MakeSharder()

	// db.DLPrintf("SHARDER", "Sharder: %v\n", sh.args)
	log.Printf("SHARDER Sharder: %v\n", sh.args)

	// XXX set removeWatch on KVs? maybe in KV

	sh.restart()

	// Must have have sh.conf by here; Add() uses it

	switch sh.args[0] {
	case "crash1", "crash2", "crash3":
		sh.Add()
	case "add":
		sh.Add()
	case "del":
		sh.Del()
	case "restart":
		return
	default:
		log.Fatalf("Unknown command %v\n", sh.args[0])
	}

	sh.nextConf = sh.balance()
	db.DLPrintf("SHARDER", "Sharder conf %v next conf: %v %v\n", sh.conf, sh.nextConf, sh.nextKvs)

	// The to-be-deleted KV must ack too
	if sh.args[0] == "del" {
		sh.nextKvs = append(sh.nextKvs, sh.args[1:]...)

	}
	sh.nkvd = len(sh.nextKvs)

	if sh.args[0] == "crash1" {
		db.DLPrintf("SHARDER", "Crash1\n")
		os.Exit(1)
	}

	log.Printf("sharder prepare\n")

	sh.prepare()

	log.Printf("sharder commit\n")

	sh.commit(0)

	sh.Exit()
}
