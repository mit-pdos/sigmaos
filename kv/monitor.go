package kv

import (
	"log"
	"os"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/depproc"
	"ulambda/fslib"
	"ulambda/memfsd"
	"ulambda/proc"
	"ulambda/stats"
)

const (
	KV        = "bin/user/kv"
	KVMONLOCK = "monlock"
)

type Monitor struct {
	mu sync.Mutex
	*fslib.FsLib
	*depproc.DepProcCtl
	pid  string
	kv   string
	args []string
}

func MakeMonitor(args []string) (*Monitor, error) {
	mo := &Monitor{}
	mo.pid = args[0]
	mo.FsLib = fslib.MakeFsLib(mo.pid)
	mo.DepProcCtl = depproc.MakeDepProcCtl(mo.FsLib, depproc.DEFAULT_JOB_ID)
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

func spawnBalancerPid(sched *depproc.DepProcCtl, opcode, pid1, pid2 string) {
	t := depproc.MakeDepProc()
	t.Pid = pid2
	t.Program = "bin/user/balancer"
	t.Args = []string{opcode, pid1}
	t.Dependencies = &depproc.Deps{map[string]bool{pid1: false}, nil}
	t.Type = proc.T_LC
	sched.Spawn(t)
}

func spawnBalancer(sched *depproc.DepProcCtl, opcode, pid1 string) string {
	t := depproc.MakeDepProc()
	t.Pid = fslib.GenPid()
	t.Program = "bin/user/balancer"
	t.Args = []string{opcode, pid1}
	t.Dependencies = &depproc.Deps{map[string]bool{pid1: false}, nil}
	t.Type = proc.T_LC
	sched.Spawn(t)
	return t.Pid
}

func spawnKVPid(sched *depproc.DepProcCtl, pid1 string, pid2 string) {
	t := depproc.MakeDepProc()
	t.Pid = pid1
	t.Program = KV
	t.Args = []string{""}
	t.Dependencies = &depproc.Deps{map[string]bool{pid1: false}, nil}
	t.Type = proc.T_LC
	sched.Spawn(t)
}

func SpawnKV(sched *depproc.DepProcCtl) string {
	t := depproc.MakeDepProc()
	t.Pid = fslib.GenPid()
	t.Program = KV
	t.Args = []string{""}
	t.Type = proc.T_LC
	sched.Spawn(t)
	return t.Pid
}

func runBalancerPid(sched *depproc.DepProcCtl, opcode, pid1, pid2 string) {
	spawnBalancerPid(sched, opcode, pid1, pid2)
	err := sched.WaitExit(pid2)
	if err != nil {
		log.Printf("runBalancer: err %v\n", err)
	}
}

func RunBalancer(sched *depproc.DepProcCtl, opcode, pid1 string) {
	pid2 := spawnBalancer(sched, opcode, pid1)
	err := sched.WaitExit(pid2)
	if err != nil {
		log.Printf("runBalancer: err %v\n", err)
	}
}

func (mo *Monitor) grow() {
	pid1 := fslib.GenPid()
	pid2 := fslib.GenPid()
	spawnKVPid(mo.DepProcCtl, pid1, pid2)
	runBalancerPid(mo.DepProcCtl, "add", pid1, pid2)
}

func (mo *Monitor) shrink(kv string) {
	RunBalancer(mo.DepProcCtl, "del", kv)
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

func (mo *Monitor) Exit() {
	mo.Exited(mo.pid)
}
