package kv

import (
	"log"
	"os"
	"sync"
	"time"

	"ulambda/fslib"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/stats"
)

const (
	MAXLOAD float64 = 85.0
	MINLOAD float64 = 40.0
)

const MS = 100 // 100 ms

type Monitor struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	ch chan bool
}

func MakeMonitor(fslib *fslib.FsLib, pclnt *procclnt.ProcClnt) *Monitor {
	mo := &Monitor{}
	mo.FsLib = fslib
	mo.ProcClnt = pclnt
	mo.ch = make(chan bool)
	go mo.monitor()
	return mo
}

func SpawnMemFS(pclnt *procclnt.ProcClnt) string {
	p := proc.MakeProc("bin/user/memfsd", []string{""})
	p.Type = proc.T_LC
	pclnt.Spawn(p)
	return p.Pid
}

func (mo *Monitor) monitor() {
	for true {
		select {
		case <-mo.ch:
			return
		default:
			time.Sleep(time.Duration(MS) * time.Millisecond)
			mo.doMonitor()
		}
	}
}

func (mo *Monitor) Done() {
	mo.ch <- true
}

func (mo *Monitor) grow() {
	pid1 := SpawnMemFS(mo.ProcClnt)
	err := mo.ProcClnt.WaitStart(pid1)
	if err != nil {
		log.Printf("runBalancer: err %v\n", err)
	}
	BalancerOp(mo.FsLib, "add", pid1)
}

func (mo *Monitor) shrink(kv string) {
	BalancerOp(mo.FsLib, "del", kv)
	n := named.MEMFS + "/" + kv + "/"
	err := mo.Evict(kv)
	if err != nil {
		log.Printf("shrink: remove %v failed %v\n", n, err)
	}
}

// XXX Use load too?
func (mo *Monitor) doMonitor() {
	var conf *Config
	for true {
		c, err := readConfig(mo.FsLib, KVCONFIG)
		if err != nil {
			log.Printf("readConfig: err %v\n", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		conf = c
		break
	}

	now := time.Now().UnixNano()
	if (now-conf.Ctime)/1_000_000_000 < 1 {
		log.Printf("Monitor: skip\n")
		return
	}

	kvs := makeKvs(conf.Shards)
	log.Printf("Monitor config %v\n", kvs)

	util := float64(0)
	low := float64(100.0)
	lowkv := ""
	var lowload stats.Tload
	n := 0
	for kv, _ := range kvs.set {
		kvd := named.MEMFS + "/" + kv + "/statsd"
		sti := stats.StatInfo{}
		err := mo.ReadFileJson(kvd, &sti)
		if err != nil {
			log.Printf("ReadFileJson failed %v\n", err)
			os.Exit(1)
		}
		n += 1
		util += sti.Util
		if sti.Util < low {
			low = sti.Util
			lowkv = kv
			lowload = sti.Load
		}
		log.Printf("path %v\n", sti.SortPath()[0:3])
	}
	util = util / float64(n)
	log.Printf("monitor: avg util %.1f low %.1f kv %v %v\n", util, low, lowkv, lowload)
	if util >= MAXLOAD {
		mo.grow()
	}
	if util < MINLOAD && len(kvs.set) > 1 {
		mo.shrink(lowkv)
	}
}
