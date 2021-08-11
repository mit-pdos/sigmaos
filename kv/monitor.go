package kv

import (
	"log"
	"os"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	"ulambda/proc"
	"ulambda/stats"
)

const (
	KV        = "bin/kv"
	KVMONLOCK = "monlock"
)

type Monitor struct {
	mu sync.Mutex
	*fslib.FsLib
	*proc.ProcCtl
	pid  string
	kv   string
	args []string
}

func MakeMonitor(args []string) (*Monitor, error) {
	mo := &Monitor{}
	mo.pid = args[0]
	mo.FsLib = fslib.MakeFsLib(mo.pid)
	mo.ProcCtl = proc.MakeProcCtl(mo.FsLib, mo.pid)
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

func spawnBalancerPid(pctl *proc.ProcCtl, opcode, pid1, pid2 string) {
	a := proc.Proc{}
	a.Pid = pid2
	a.Program = "bin/balancer"
	a.Args = []string{opcode, pid1}
	a.StartDep = map[string]bool{pid1: false}
	a.ExitDep = nil
	a.Type = proc.T_LC
	pctl.Spawn(&a)
}

func spawnBalancer(pctl *proc.ProcCtl, opcode, pid1 string) string {
	a := proc.Proc{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/balancer"
	a.Args = []string{opcode, pid1}
	a.StartDep = map[string]bool{pid1: false}
	a.ExitDep = nil
	a.Type = proc.T_LC
	pctl.Spawn(&a)
	return a.Pid
}

func spawnKVPid(pctl *proc.ProcCtl, pid1 string, pid2 string) {
	a := proc.Proc{}
	a.Pid = pid1
	a.Program = KV
	a.Args = []string{""}
	a.StartDep = map[string]bool{pid1: false}
	a.ExitDep = nil
	a.Type = proc.T_LC
	pctl.Spawn(&a)
}

func SpawnKV(pctl *proc.ProcCtl) string {
	a := proc.Proc{}
	a.Pid = fslib.GenPid()
	a.Program = KV
	a.Args = []string{""}
	a.StartDep = nil
	a.ExitDep = nil
	a.Type = proc.T_LC
	pctl.Spawn(&a)
	return a.Pid
}

func runBalancerPid(pctl *proc.ProcCtl, opcode, pid1, pid2 string) {
	spawnBalancerPid(pctl, opcode, pid1, pid2)
	ok, err := pctl.Wait(pid2)
	if string(ok) != "OK" || err != nil {
		log.Printf("runBalancer: ok %v err %v\n", string(ok), err)
	}
}

func RunBalancer(pctl *proc.ProcCtl, opcode, pid1 string) {
	pid2 := spawnBalancer(pctl, opcode, pid1)
	ok, err := pctl.Wait(pid2)
	if string(ok) != "OK" || err != nil {
		log.Printf("runBalancer: ok %v err %v\n", string(ok), err)
	}
}

// See if there is KV waiting to be run
func (mo *Monitor) kvwaiting() bool {
	jobs, err := mo.ReadDir(proc.WAITQ)
	if err != nil {
		log.Fatalf("grow: cannot read runq err %v\n", err)
	}
	for _, j := range jobs {
		log.Printf("job %v\n", j.Name)
		p, err := mo.GetProcFile(proc.WAITQ, j.Name)
		if err != nil {
			log.Printf("Error getting proc file in Monitor.kvwaiting: %v", err)
		}
		log.Printf("proc %v\n", p)
		if p.Program == KV {
			return true
		}
	}
	return false
}

func (mo *Monitor) grow() {
	pid1 := fslib.GenPid()
	pid2 := fslib.GenPid()
	spawnKVPid(mo.ProcCtl, pid1, pid2)
	runBalancerPid(mo.ProcCtl, "add", pid1, pid2)
}

func (mo *Monitor) shrink(kv string) {
	RunBalancer(mo.ProcCtl, "del", kv)
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
