package kv

import (
	"log"
	"os"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	npo "ulambda/npobjsrv"
)

type Monitor struct {
	mu sync.Mutex
	*fslib.FsLib
	pid   string
	kv    string
	args  []string
	conf2 *Config2
}

func MakeMonitor(args []string) (*Monitor, error) {
	mo := &Monitor{}
	mo.pid = args[0]
	mo.FsLib = fslib.MakeFsLib(mo.pid)
	db.Name(mo.pid)
	mo.Started(mo.pid)
	return mo, nil
}

func (mo *Monitor) Work() {
	sts, err := mo.ReadDir("name/memfsd")
	if err != nil {
		log.Printf("Readdir failed %v\n", err)
		os.Exit(1)
	}
	for _, st := range sts {
		kvd := "name/memfsd/" + st.Name + "/statsd"
		log.Printf("kv: %v\n", kvd)
		stats := npo.Stats{}
		err := mo.ReadFileJson(kvd, &stats)
		if err != nil {
			log.Printf("ReadFileJson failed %v\n", err)
			os.Exit(1)
		}
		log.Printf("stats:\n%v\n", stats)
	}
}
