package kv

//
// Shard coordinator: assigns shards to KVs.  Assumes no KV failures
// This is a short-lived daemon: it rebalances shards and then exists.
//

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/memfsd"
	np "ulambda/ninep"
)

const (
	NSHARD       = 10
	KVDIR        = "name/kv"
	SHARDER      = KVDIR + "/sharder"
	KVCONFIG     = KVDIR + "/config"
	KVNEXTCONFIG = KVDIR + "/nextconfig"
)

var ErrWrongKv = errors.New("ErrWrongKv")
var ErrRetry = errors.New("ErrRetry")

type SharderDev struct {
	sh *Sharder
}

func (shdev *SharderDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	var err error
	if strings.HasPrefix(t, "Prepared") {
		err = shdev.sh.prepared(t[len("Prepared "):])
	} else if strings.HasPrefix(t, "Exit") {
		shdev.sh.exit()
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}
	return np.Tsize(len(data)), err
}

func (shdev *SharderDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	return nil, errors.New("Not support")
}

func (shdev *SharderDev) Len() np.Tlength {
	return 0
}

type Config struct {
	N      int
	Shards []string // maps shard # to server
}

func makeConfig(n int) *Config {
	cf := &Config{n, make([]string, NSHARD)}
	return cf
}

type Sharder struct {
	mu   sync.Mutex
	cond *sync.Cond
	*fslib.FsLibSrv
	pid      string
	bin      string
	args     []string
	kvs      []string // the kv servers in this configuration
	nextKvs  []string // the available kvs for the next config
	nkvd     int      // # KVs in reconfiguration
	conf     *Config
	nextConf *Config
	done     bool
}

func MakeSharder(args []string) (*Sharder, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("MakeSharder: too few arguments %v\n", args)
	}
	log.Printf("Sharder: %v\n", args)
	sh := &Sharder{}
	sh.cond = sync.NewCond(&sh.mu)
	sh.pid = args[0]
	sh.bin = args[1]
	sh.args = args[2:]
	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("MakeSharder: no IP %v\n", err)
	}
	fsd := memfsd.MakeFsd("sharder", ip+":0", nil)
	fls, err := fslib.InitFs(SHARDER, fsd, &SharderDev{sh})
	if err != nil {
		return nil, err
	}
	sh.FsLibSrv = fls
	sh.Started(sh.pid)

	db.SetDebug(false)

	return sh, nil
}

func (sh *Sharder) exit() error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	db.DPrintf("Exit %v\n", sh.pid)
	sh.done = true
	sh.nextKvs = make([]string, 0)
	sh.cond.Signal()
	return nil
}

func (sh *Sharder) prepared(kvd string) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	db.DPrintf("Prepared: %v\n", kvd)
	sh.nkvd -= 1
	if sh.nkvd <= 0 {
		sh.cond.Signal()

	}
	return nil
}

// Tell kv prepare to reconfigure
func (sh *Sharder) prepare(kv string) {
	sh.mu.Unlock()
	defer sh.mu.Lock()

	dev := KVDIR + "/" + kv + "/dev"
	err := sh.WriteFile(dev, []byte("Prepare"))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", dev, err)
	}

}

// Tell kv commit to reconfigure
func (sh *Sharder) commit(kv string) {
	sh.mu.Unlock()
	defer sh.mu.Lock()

	dev := KVDIR + "/" + kv + "/dev"
	err := sh.WriteFile(dev, []byte("Commit"))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", dev, err)
	}

}

// Caller holds lock
// XXX minimize movement
func (sh *Sharder) balance() *Config {
	j := 0
	conf := makeConfig(sh.conf.N + 1)

	db.DPrintf("shards %v (len %v) kvs %v\n", sh.conf.Shards,
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
	sh.Exiting(sh.pid, "OK")
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

func (sh *Sharder) Init() {
	sh.conf = makeConfig(0)
	err := sh.MakeFileJson(KVCONFIG, *sh.conf)
	if err != nil {
		log.Fatalf("Sharder: cannot make file  %v %v\n", KVCONFIG, err)
	}
}

func (sh *Sharder) Work() {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.conf = sh.readConfig(KVCONFIG)
	if sh.conf == nil {
		sh.Init()
	}

	// log.Printf("Sharder: %v %v\n", sh.conf, sh.args)
	if sh.args[0] == "add" {
		sh.nextKvs = append(sh.kvs, sh.args[1:]...)
	} else {
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

	sh.nextConf = sh.balance()
	db.DPrintf("Sharder next conf: %v %v\n", sh.nextConf, sh.nextKvs)
	err := sh.MakeFileJson(KVNEXTCONFIG, *sh.nextConf)
	if err != nil {
		log.Printf("Sharder: %v error %v\n", KVNEXTCONFIG, err)
		return
	}

	if sh.args[0] == "del" {
		sh.nextKvs = append(sh.nextKvs, sh.args[1:]...)
	}

	sh.nkvd = len(sh.nextKvs)
	for _, kv := range sh.nextKvs {
		sh.prepare(kv)
	}

	// XXX handle crashed KVs
	for sh.nkvd > 0 {
		sh.cond.Wait()

	}

	log.Printf("Commit to %v\n", sh.nextConf)
	// commit to new config
	err = sh.Rename(KVNEXTCONFIG, KVCONFIG)
	if err != nil {
		log.Fatalf("Sharder: rename %v -> %v: error %v\n",
			KVNEXTCONFIG, KVCONFIG, err)
	}
	for _, kv := range sh.nextKvs {
		sh.commit(kv)
	}
}
