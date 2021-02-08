package kv

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
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
	kvs      []string // the kv servers in this configuration
	nextKvs  []string // the available kvs for the next config
	conf     *Config
	nextConf *Config
	nkvd     int // # KVs in reconfiguration
	done     bool
}

func MakeSharder(args []string) (*Sharder, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("MakeSharder: too few arguments %v\n", args)
	}
	log.Printf("Sharder: %v\n", args)
	sh := &Sharder{}
	sh.cond = sync.NewCond(&sh.mu)
	sh.pid = args[0]
	sh.bin = args[1]
	fls, err := fslib.InitFs(SHARDER, &SharderDev{sh})
	if err != nil {
		return nil, err
	}
	sh.FsLibSrv = fls
	sh.Started(sh.pid)

	db.SetDebug(true)

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

	log.Printf("Prepared: %v\n", kvd)
	sh.nkvd -= 1
	if sh.nkvd <= 0 {
		sh.cond.Signal()

	}
	return nil
}

// Tell kv prepare to reconfigure
func (sh *Sharder) prepare(kv string) {
	dev := kv + "/dev"
	err := sh.WriteFile(dev, []byte("Prepare"))
	if err != nil {
		db.DPrintf("WriteFile: %v %v\n", dev, err)
	}

}

// Tell kv commit to reconfigure
func (sh *Sharder) commit(kv string) {
	dev := kv + "/dev"
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
	sh.Exiting(sh.pid)
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
		return
	}
	log.Printf("Sharder work %v\n", sh.conf)
	sh.nextKvs = make([]string, 0)
	sh.ProcessDir(KVDIR, func(st *np.Stat) (bool, error) {
		if strings.HasPrefix(st.Name, "kv-") {
			sh.nextKvs = append(sh.nextKvs, st.Name)
		}
		return false, nil
	})

	log.Printf("kv.conf %v nextKvs %v\n", sh.conf, sh.nextKvs)

	sh.nextConf = sh.balance()
	log.Printf("Sharder next conf: %v %v\n", sh.nextConf, sh.kvs)
	err := sh.MakeFileJson(KVNEXTCONFIG, *sh.nextConf)
	if err != nil {
		log.Printf("Sharder: %v error %v\n", KVNEXTCONFIG, err)
		return
	}

	sh.nkvd = len(sh.kvs)
	log.Printf("nkvd %v\n", sh.nkvd)
	for _, kv := range sh.kvs {
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
		log.Printf("Work: rename %v -> %v: %v\n", KVNEXTCONFIG, KVCONFIG, err)
	}
	for _, kv := range sh.kvs {
		sh.commit(kv)
	}
}
