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
)

const (
	NSHARD       = 10
	KVDIR        = "name/kv"
	SHARDER      = KVDIR + "/sharder"
	KVCONFIG     = KVDIR + "/config"
	KVCONFIGTMP  = KVDIR + "/configtmp"
	KVNEXTCONFIG = KVDIR + "/nextconfig"
	KVPREPARED   = KVDIR + "/prepared/"
	KVCOMMITTED  = KVDIR + "/committed/"
	KVLOCK       = "lock"
)

type Tstatus int

const (
	COMMIT Tstatus = 0
	ABORT  Tstatus = 1
	CRASH  Tstatus = 2
)

func (s Tstatus) String() string {
	switch s {
	case COMMIT:
		return "COMMIT"
	case ABORT:
		return "ABORT"
	default:
		return "CRASH"
	}
}

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
	ch       chan Tstatus
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
	sh.ch = make(chan Tstatus)
	sh.pid = args[0]
	sh.args = args[1:]
	db.Name("sharder")

	// Grab KVLOCK before starting sharder
	fsl := fslib.MakeFsLib(SHARDER)
	if err := fsl.LockFile(KVDIR, KVLOCK); err != nil {
		log.Fatalf("Lock failed %v\n", err)
	}

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

func (sh *Sharder) unlock() {
	log.Printf("SHARDER unlock\n")
	if err := sh.UnlockFile(KVDIR, KVLOCK); err != nil {
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

func (sh *Sharder) doCommit(nextConf *Config, prepared map[string]bool) bool {
	kvds := make(map[string]bool)
	for _, kv := range nextConf.Shards {
		if _, ok := kvds[kv]; !ok {
			kvds[kv] = true
		}
	}
	if prepared == nil || len(prepared) != len(kvds) {
		return false
	}
	for kv, _ := range kvds {
		if _, ok := prepared[kv]; !ok {
			return false
		}
	}
	return true
}

func (sh *Sharder) restart() {
	sh.conf = sh.readConfig(KVCONFIG)
	sh.nextConf = sh.readConfig(KVNEXTCONFIG)
	prepared := sh.readStatus(KVPREPARED)
	committed := sh.readStatus(KVCOMMITTED)

	db.DLPrintf("SHARDER", "Restart: conf %v next %v prepared %v commit %v\n",
		sh.conf, sh.nextConf, prepared, committed)

	if sh.nextConf == nil {
		// either commit/aborted or never started
		db.DLPrintf("SHARDER", "Restart: clean\n")
		return
	}
	todo := sh.nkvd - len(committed)
	if sh.doCommit(sh.nextConf, prepared) {
		db.DLPrintf("SHARDER", "Restart: finish commit %d\n", todo)
		sh.commit(todo, true)
	} else {
		db.DLPrintf("SHARDER", "Restart: abort\n")
		sh.commit(todo, false)
	}

	// we committed/aborted; reread sh.conf to
	// get new this config
	sh.conf = sh.readConfig(KVCONFIG)
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

func (sh *Sharder) initShards(exclKvs []string) bool {
	db.DLPrintf("SHARDER", "initShards %v\n", exclKvs)
	excl := make(map[string]bool)
	for _, kv := range exclKvs {
		excl[kv] = true
	}
	for s, kv := range sh.conf.Shards {
		if _, ok := excl[kv]; ok { // shard s has been lost
			kvd := sh.nextConf.Shards[s]
			dst := shardPath(kvd, s, sh.nextConf.N)
			db.DLPrintf("SHARDER: Init shard dir %v\n", dst)
			err := sh.Mkdir(dst, 0777)
			if err != nil {
				db.DLPrintf("KV", "initShards: mkdir %v err %v\n", dst, err)
				return false
			}
		}
	}
	return true
}

func (sh *Sharder) rmStatusFiles(dir string) {
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

func (sh *Sharder) watchStatus(p string, err error) {
	db.DLPrintf("SHARDER", "watchStatus %v\n", p)
	status := ABORT
	b, err := sh.ReadFile(p)
	if err != nil {
		db.DLPrintf("SHARDER", "watchStatus ReadFile %v err %v\n", p, b)
	}
	if string(b) == "OK" {
		status = COMMIT
	}
	sh.ch <- status
}

func (sh *Sharder) setStatusWatches(dir string) {
	for _, kv := range sh.nextKvs {
		fn := dir + kv
		// set watch for existence of fn, which indicates fn
		// has prepared/committed
		_, err := sh.ReadFileWatch(fn, sh.watchStatus)
		if err == nil {
			log.Fatalf("SHARDER: set status watch failed %v", err)
		}
	}
}

func (sh *Sharder) watchKV(p string, err error) {
	db.DLPrintf("SHARDER", "watchKV %v\n", p)
	sh.ch <- CRASH
}

func (sh *Sharder) setKVWatches() {
	for _, kv := range sh.nextKvs {
		// set watch for KV, in case it crashes during 2PC
		err := sh.SetRemoveWatch(KVDIR+"/"+kv, sh.watchKV)
		if err != nil {
			log.Fatalf("SHARDER: set KV watch failed %v", err)
		}
	}
}

func (sh *Sharder) prepare() (bool, int) {
	sh.Remove(KVCONFIGTMP) // don't care if succeeds or not
	sh.rmStatusFiles(KVPREPARED)
	sh.rmStatusFiles(KVCOMMITTED)

	sh.setKVWatches()
	sh.setStatusWatches(KVPREPARED)

	err := sh.MakeFileJsonAtomic(KVNEXTCONFIG, 0777, *sh.nextConf)
	if err != nil {
		db.DLPrintf("SHARDER", "SHARDER: MakeFileJsonAtomic %v err %v\n",
			KVNEXTCONFIG, err)
	}

	// depending how many KVs ack, crash2 results
	// in a abort or commit
	if sh.args[0] == "crash2" {
		db.DLPrintf("SHARDER", "Crash2\n")
		os.Exit(1)
	}

	success := true
	n := 0
	for i := 0; i < sh.nkvd; i++ {
		status := <-sh.ch
		switch status {
		case COMMIT:
			db.DLPrintf("SHARDER", "KV prepared\n")
			n += 1
		case ABORT:
			db.DLPrintf("SHARDER", "KV aborted\n")
			n += 1
			success = false
		default:
			db.DLPrintf("SHARDER", "KV crashed\n")
			success = false
		}
	}
	return success, n
}

func (sh *Sharder) commit(nfollower int, ok bool) {
	if ok {
		db.DLPrintf("SHARDER", "Commit to %v\n", sh.nextConf)
	} else {
		// Rename KVCONFIGTMP into KVNEXTCONFIG so that the followers
		// will abort to the old KVCONFIG
		if err := sh.CopyFile(KVCONFIG, KVCONFIGTMP); err != nil {
			db.DLPrintf("SHARDER", "CopyFile failed %v\n", err)
		}
		err := sh.Rename(KVCONFIGTMP, KVNEXTCONFIG)
		if err != nil {
			db.DLPrintf("SHARDER", "SHARDER: rename %v -> %v: error %v\n",
				KVCONFIGTMP, KVNEXTCONFIG, err)
			return
		}
		db.DLPrintf("SHARDER", "Abort to %v\n", sh.conf)
	}

	sh.setStatusWatches(KVCOMMITTED)

	// commit/abort to new KVCONFIG, which maybe the same as the
	// old one
	err := sh.Rename(KVNEXTCONFIG, KVCONFIG)
	if err != nil {
		db.DLPrintf("SHARDER", "SHARDER: rename %v -> %v: error %v\n",
			KVNEXTCONFIG, KVCONFIG, err)
		return
	}

	// crash3 should results in commit (assuming no KVs crash)
	if sh.args[0] == "crash3" {
		db.DLPrintf("SHARDER", "Crash3\n")
		os.Exit(1)
	}

	for i := 0; i < nfollower; i++ {
		s := <-sh.ch
		db.DLPrintf("SHARDER", "KV commit status %v\n", s)
	}

	db.DLPrintf("SHARDER", "Done commit/abort\n")
}

func (sh *Sharder) TwoPC() {
	defer sh.unlock() // release lock acquired in MakeSharder()

	// db.DLPrintf("SHARDER", "Sharder: %v\n", sh.args)
	log.Printf("SHARDER Sharder: %v\n", sh.args)

	// XXX set removeWatch on KVs? maybe in KV

	sh.restart()

	// Must have have sh.conf by here; Add() uses it

	switch sh.args[0] {
	case "crash1", "crash2", "crash3", "crash4", "crash5":
		sh.Add()
	case "add":
		sh.Add()
	case "del":
		sh.Del()
	case "excl":
		sh.Del()
	case "restart":
		return
	default:
		log.Fatalf("Unknown command %v\n", sh.args[0])
	}

	sh.nextConf = sh.balance()
	db.DLPrintf("SHARDER", "Sharder conf %v next conf: %v %v\n", sh.conf, sh.nextConf, sh.nextKvs)

	// A gracefully exiting KV must ack too. We add it back here
	// after balance() without it.
	if sh.args[0] == "del" {
		sh.nextKvs = append(sh.nextKvs, sh.args[1:]...)

	}
	sh.nkvd = len(sh.nextKvs)

	if sh.args[0] == "crash1" {
		db.DLPrintf("SHARDER", "Crash1\n")
		os.Exit(1)
	}

	log.Printf("SHARDER prepare\n")

	ok, n := sh.prepare()

	log.Printf("SHARDER commit/abort %v\n", ok)

	if ok && sh.args[0] == "excl" {
		// make empty shards for the ones we lost; if it fails
		// abort 2PC.
		ok = sh.initShards(sh.args[1:])
	}

	sh.commit(n, ok)

	sh.Exit()
}
