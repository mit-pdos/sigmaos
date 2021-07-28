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

func spawnBalancerPid(fsl *fslib.FsLib, opcode, pid1, pid2 string) {
	a := fslib.Attr{}
	a.Pid = pid2
	a.Program = "bin/balancer"
	a.Args = []string{opcode, pid1}
	a.PairDep = []fslib.PDep{fslib.PDep{pid1, pid2}}
	a.ExitDep = nil
	a.Type = fslib.T_LC
	fsl.Spawn(&a)
}

func spawnBalancer(fsl *fslib.FsLib, opcode, pid1 string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/balancer"
	a.Args = []string{opcode, pid1}
	a.PairDep = []fslib.PDep{fslib.PDep{a.Pid, pid1}}
	a.ExitDep = nil
	a.Type = fslib.T_LC
	fsl.Spawn(&a)
	return a.Pid
}

func spawnKVPid(fsl *fslib.FsLib, pid1 string, pid2 string) {
	a := fslib.Attr{}
	a.Pid = pid1
	a.Program = KV
	a.Args = []string{""}
	a.PairDep = []fslib.PDep{fslib.PDep{pid1, pid2}}
	a.ExitDep = nil
	a.Type = fslib.T_LC
	fsl.Spawn(&a)
}

func SpawnKV(fsl *fslib.FsLib) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = KV
	a.Args = []string{""}
	a.PairDep = nil
	a.ExitDep = nil
	a.Type = fslib.T_LC
	fsl.Spawn(&a)
	return a.Pid
}

func runBalancerPid(fsl *fslib.FsLib, opcode, pid1, pid2 string) {
	spawnBalancerPid(fsl, opcode, pid1, pid2)
	ok, err := fsl.Wait(pid2)
	if string(ok) != "OK" || err != nil {
		log.Printf("runBalancer: ok %v err %v\n", string(ok), err)
	}
}

func RunBalancer(fsl *fslib.FsLib, opcode, pid1 string) {
	pid2 := spawnBalancer(fsl, opcode, pid1)
	ok, err := fsl.Wait(pid2)
	if string(ok) != "OK" || err != nil {
		log.Printf("runBalancer: ok %v err %v\n", string(ok), err)
	}
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
	pid1 := fslib.GenPid()
	pid2 := fslib.GenPid()
	spawnKVPid(mo.FsLib, pid1, pid2)
	runBalancerPid(mo.FsLib, "add", pid1, pid2)
}

func (mo *Monitor) shrink(kv string) {
	RunBalancer(mo.FsLib, "del", kv)
	err := mo.Remove(memfsd.MEMFS + "/" + kv + "/")
	if err != nil {
		log.Printf("shrink: remove failed %v\n", err)
	}
}

// XXX Use load too?
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
			lowload = sti.Load
		}
		log.Printf("path %v\n", sti.SortPath()[0:3])
	}
	util = util / float64(n)
	log.Printf("monitor: avg util %.1f low %.1f kv %v %v\n", util, low, lowkv, lowload)
	if util >= stats.MAXLOAD {
		mo.grow()
	}
	if util < stats.MINLOAD && len(kvs.set) > 1 {
		mo.shrink(lowkv)
	}
}
