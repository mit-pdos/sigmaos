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
	NSHARDS  = 5
	KVCONFIG = SHARDER + "/config"
)

var ErrWrongKv = errors.New("ErrWrongKv")
var ErrRetry = errors.New("ErrRetry")

type SharderDev struct {
	sh *Sharder
}

func (shdev *SharderDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	log.Printf("SharderDev.write %v\n", t)
	if strings.HasPrefix(t, "Join") {
		shdev.sh.join(t[len("Join "):])
	} else if strings.HasPrefix(t, "Leave") {
		shdev.sh.leave(t[len("Leave"):])
	} else if strings.HasPrefix(t, "Add") {
		shdev.sh.add()
	} else if strings.HasPrefix(t, "Resume") {
		shdev.sh.resume(t[len("Resume "):])
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}
	return np.Tsize(len(data)), nil
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

func makeConfig() *Config {
	cf := &Config{0, make([]string, NSHARDS)}
	for i := 0; i < NSHARDS; i++ {
		cf.Shards = append(cf.Shards, "")
	}
	return cf
}

type Sharder struct {
	mu   sync.Mutex
	cond *sync.Cond
	*fslib.FsLibSrv
	pid  string
	kvs  []string // the available kv servers
	conf *Config
	nkvd int // # KVs in reconfiguration
}

func MakeSharder(args []string) (*Sharder, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("MakeSharder: too few arguments %v\n", args)
	}
	log.Printf("Sharder: %v\n", args)
	sh := &Sharder{}
	sh.cond = sync.NewCond(&sh.mu)
	sh.conf = makeConfig()
	sh.kvs = make([]string, 0)
	sh.pid = args[0]
	fls, err := fslib.InitFs(SHARDER, &SharderDev{sh})
	if err != nil {
		return nil, err
	}

	err = fls.MakeFileJson(KVCONFIG, *sh.conf)
	if err != nil {
		return nil, err
	}
	sh.FsLibSrv = fls
	db.SetDebug(false)
	sh.Started(sh.pid)
	return sh, nil
}

func (sh *Sharder) add() {
	sh.spawnKv()
}

func (sh *Sharder) join(kvd string) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	log.Printf("join:%v\n", kvd)
	sh.kvs = append(sh.kvs, kvd)
	sh.cond.Signal()
}

func (sh *Sharder) leave(kvd string) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	for i, v := range sh.kvs {
		if v == kvd {
			sh.kvs = append(sh.kvs[:i], sh.kvs[i+1:]...)
			break
		}
	}
	sh.cond.Signal()
}

func (sh *Sharder) resume(kvd string) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.nkvd -= 1
	if sh.nkvd <= 0 {
		log.Printf("All KVs resumed\n")
	}
}

// Tell kv to reconfigure
func (sh *Sharder) refresh(kv string) {
	dev := kv + "/dev"
	err := sh.WriteFile(dev, []byte("Reconfigure"))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", dev, err)
	}

}

// Caller holds lock
// XXX minimize movement
func (sh *Sharder) balance() {
	j := 0
	sh.conf.N = sh.conf.N + 1
	log.Printf("shards %v kvs %v\n", sh.conf.Shards, sh.kvs)
	for i, _ := range sh.conf.Shards {
		sh.conf.Shards[i] = sh.kvs[j]
		j = (j + 1) % len(sh.kvs)
	}
}

func (sh *Sharder) Exit() {
	sh.Exiting(sh.pid)
}

func (sh *Sharder) spawnKv() error {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "./bin/kvd"
	a.Args = []string{}
	a.PairDep = []fslib.PDep{fslib.PDep{sh.pid, a.Pid}}
	a.ExitDep = nil
	return sh.Spawn(&a)
}

// XXX Support shrinking
// XXX Add/shrink dynamically
// XXX setup pairdeps between kvs so that scheduler can gang schedule them?
// XXX Handle failed kvs
func (sh *Sharder) Work() {
	sh.mu.Lock()
	sh.spawnKv()
	for {
		sh.cond.Wait()
		if sh.nkvd == 0 {
			sh.balance()
			log.Printf("sharder conf: %v\n", sh.conf)
			err := sh.WriteFileJson(KVCONFIG, *sh.conf)
			if err != nil {
				log.Printf("add write error %v\n", err)
				return
			}
			sh.nkvd = len(sh.kvs)
			for _, kv := range sh.kvs {
				sh.refresh(kv)
			}
		} else {
			log.Printf("sharder: rebalancing nkvd %v\n", sh.nkvd)
		}
	}
}
