package kvlambda

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

const (
	NSHARDS = 100
)

type SharderDev struct {
	sh *Sharder
}

func (shdev *SharderDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	log.Printf("SharderDev.write %v\n", t)
	if strings.HasPrefix(t, "Add") {
		shard := strings.TrimLeft(t, "Add ")
		shdev.sh.add(shard)
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
	fls  *fslib.FsLibSrv
	pid  string
	kvs  []string // the available kv servers
	conf *Config
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
	fls, err := fslib.InitFs(KV+"/sharder", &SharderDev{sh})
	if err != nil {
		return nil, err
	}
	err = fls.MakeFile(KV+"/sharder/config", nil)
	if err != nil {
		return nil, err
	}
	sh.fls = fls
	db.SetDebug(false)
	sh.fls.Started(sh.pid)
	return sh, nil
}

// Caller holds lock
// XXX minimize movement
func (sh *Sharder) balance() {
	j := 0
	sh.conf.N = sh.conf.N + 1
	for i, _ := range sh.conf.Shards {
		sh.conf.Shards[i] = sh.kvs[j]
		j = (j + 1) % len(sh.kvs)
	}

}

func (sh *Sharder) add(shard string) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.kvs = append(sh.kvs, shard)
	sh.balance()
	b, err := json.Marshal(sh.conf)
	if err != nil {
		log.Fatal("add marshal error", err)
	}
	err = sh.fls.WriteFile(KV+"/sharder/config", b)
	if err != nil {
		log.Printf("add write error %v\n", err)
		return
	}
}

func (sh *Sharder) Exit() {
	sh.fls.Exiting(sh.pid)
}

func (sh *Sharder) Work() {
	sh.mu.Lock()
	for {
		sh.cond.Wait()
	}
}

// a := fslib.Attr{pid, "./bin/kvd",
// 	[]string{s + "-" + kv.krange[1],
// 		kv.krange[0] + "-" + kv.krange[1]},
// 	[]fslib.PDep{fslib.PDep{kv.pid, pid}},
// 	nil}
// err = kv.fls.Spawn(&a)
// if err != nil {
// 	log.Fatalf("Spawn failed %v\n", err)
// }
