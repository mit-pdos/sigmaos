package kv

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	"ulambda/perf"
	"ulambda/stats"
)

const (
	KV        = "bin/kv"
	KVMONLOCK = "monlock"
)

type Monitor struct {
	mu sync.Mutex
	*fslib.FsLib
	pid  string
	kv   string
	args []string
}

func MakeMonitor(args []string) (*Monitor, error) {
	mo := &Monitor{}
	mo.pid = args[0]
	mo.FsLib = fslib.MakeFsLib(mo.pid)
	db.Name(mo.pid)

	if err := mo.LockFile(KVDIR, KVMONLOCK); err != nil {
		log.Fatalf("Lock failed %v\n", err)
	}

	mo.Started(mo.pid)
	return mo, nil
}

func (mo *Monitor) unlock() {
	if err := mo.UnlockFile(KVDIR, KVMONLOCK); err != nil {
		log.Fatalf("Unlock failed failed %v\n", err)
	}

}

func spawnBalancer(fsl *fslib.FsLib, opcode, mfs string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/balancer"
	a.Args = []string{opcode, mfs}
	a.PairDep = nil
	a.ExitDep = nil
	fsl.Spawn(&a)
	return a.Pid
}

func spawnKV(fsl *fslib.FsLib) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = KV
	a.Args = []string{""}
	a.PairDep = nil
	a.ExitDep = nil
	fsl.Spawn(&a)
	return a.Pid
}

func runBalancer(fsl *fslib.FsLib, opcode, mfs string) {
	pid1 := spawnBalancer(fsl, opcode, mfs)
	ok, err := fsl.Wait(pid1)
	if string(ok) != "OK" || err != nil {
		log.Printf("runBalancer: ok %v err %v\n", string(ok), err)
	}
	log.Printf("balancer %v done\n", pid1)
}

// See if there is KV waiting to be run
func (mo *Monitor) kvwaiting() bool {
	jobs, err := mo.ReadWaitQ()
	if err != nil {
		log.Fatalf("grow: cannot read runq err %v\n", err)
	}
	for _, j := range jobs {
		log.Printf("job %v\n", j.Name)
		a, err := mo.ReadWaitQJob(j.Name)
		var attr fslib.Attr
		err = json.Unmarshal(a, &attr)
		if err != nil {
			log.Printf("grow: unmarshal err %v", err)
		}
		log.Printf("attr %v\n", attr)
		if attr.Program == KV {
			return true
		}
	}
	return false
}

func (mo *Monitor) grow() {
	pid := spawnKV(mo.FsLib)
	// XXX
	for true {
		ok := mo.HasBeenSpawned(pid)
		if ok {
			break
		}
	}
	log.Printf("kv running\n")
	runBalancer(mo.FsLib, "add", pid)
}

func (mo *Monitor) shrink(kv string) {
	runBalancer(mo.FsLib, "del", kv)
	err := mo.Remove(memfsd.MEMFS + "/" + kv + "/")
	if err != nil {
		log.Printf("shrink: remove failed %v\n", err)
	}
}

func (mo *Monitor) Work() {
	defer mo.unlock() // release lock acquired in MakeMonitor()

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
	kvs := makeKvs(conf.Shards)
	log.Printf("Monitor config %v\n", kvs)

	util := float64(0)
	low := float64(100.0)
	lowkv := ""
	n := 0
	for kv, _ := range kvs.set {
		kvd := memfsd.MEMFS + "/" + kv + "/statsd"
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
		}
	}
	util = util / float64(n)
	log.Printf("monitor: avg util %f low %f\n", util, low)
	if util >= perf.MAXLOAD {
		mo.grow()
	}
	if util < perf.MINLOAD && len(kvs.set) > 1 {
		mo.shrink(lowkv)
	}
}
