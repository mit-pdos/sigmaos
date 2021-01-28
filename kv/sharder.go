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
	KVCONFIG     = SHARDER + "/config"
	KVNEXTCONFIG = SHARDER + "/nextconfig"
)

var ErrWrongKv = errors.New("ErrWrongKv")
var ErrRetry = errors.New("ErrRetry")

type SharderDev struct {
	sh *Sharder
}

func (shdev *SharderDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	var err error
	if strings.HasPrefix(t, "Join") {
		err = shdev.sh.join(t[len("Join "):])
	} else if strings.HasPrefix(t, "Add") {
		shdev.sh.add()
	} else if strings.HasPrefix(t, "Del") {
		shdev.sh.del()
	} else if strings.HasPrefix(t, "Prepared") {
		err = shdev.sh.prepared(t[len("Prepared "):])
	} else if strings.HasPrefix(t, "Exit") {
		shdev.sh.exit()
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}
	return np.Tsize(len(data)), err
}

func (shdev *SharderDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	//	if off == 0 {
	//	s := shdev.sd.ps()
	//return []byte(s), nil
	//}
	return nil, nil
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
	sh.conf = makeConfig(0)
	sh.kvs = make([]string, 0)
	sh.pid = args[0]
	sh.bin = args[1]
	fls, err := fslib.InitFs(SHARDER, &SharderDev{sh})
	if err != nil {
		return nil, err
	}
	err = fls.MakeFileJson(KVCONFIG, *sh.conf)
	if err != nil {
		return nil, err
	}
	sh.FsLibSrv = fls
	sh.Started(sh.pid)
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

// Add a new KV, which will invoke join once
// it is running.
func (sh *Sharder) add() {
	sh.spawnKv()
}

// Remove one KV
func (sh *Sharder) del() error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	db.DPrintf("Del: %v\n", sh.kvs[0])
	sh.nextKvs = sh.kvs[1:]
	sh.cond.Signal()
	return nil
}

func (sh *Sharder) join(kvd string) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	db.DPrintf("Join: %v\n", kvd)
	if sh.nextConf != nil {
		return fmt.Errorf("In reconfiguration %v -> %v\n",
			sh.conf.N, sh.nextConf.N)
	}
	sh.nextKvs = append(sh.kvs, kvd)
	sh.kvs = append(sh.kvs, kvd)
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

func (sh *Sharder) spawnKv() error {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = sh.bin + "/kvd"
	a.Args = []string{}
	a.PairDep = []fslib.PDep{fslib.PDep{sh.pid, a.Pid}}
	a.ExitDep = nil
	return sh.Spawn(&a)
}

// XXX Handle failed kvs
func (sh *Sharder) Work() {
	sh.mu.Lock()
	err := sh.spawnKv()
	if err != nil {
		log.Fatalf("spawnKv: error %v\n", err)
	}
	for !(sh.done && sh.nextConf == nil) {

		sh.cond.Wait()

		if sh.nextConf == nil {
			sh.nextConf = sh.balance()
			db.DPrintf("Sharder next conf: %v %v\n", sh.nextConf, sh.kvs)
			err := sh.MakeFileJson(KVNEXTCONFIG, *sh.nextConf)
			if err != nil {
				log.Printf("Work: %v error %v\n", KVNEXTCONFIG, err)
				return
			}
			sh.nkvd = len(sh.kvs)
			for _, kv := range sh.kvs {
				sh.prepare(kv)
			}
		} else {
			if sh.nkvd == 0 { // all kvs are prepared?
				db.DPrintf("Commit to %v\n", sh.nextConf)
				// commit to new config
				err := sh.Rename(KVNEXTCONFIG, KVCONFIG)
				if err != nil {
					log.Printf("Work: rename error %v\n", err)
				}
				for _, kv := range sh.kvs {
					sh.commit(kv)
				}
				sh.conf = sh.nextConf
				sh.kvs = sh.nextKvs
				sh.nextConf = nil
			} else {
				log.Printf("Sharder: reconfig in progress  %v -> %v\n",
					sh.conf.N, sh.nextConf.N)
			}

		}
	}
}
