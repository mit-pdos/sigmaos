package kv

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"ulambda/fslib"
	"ulambda/linuxsched"
	"ulambda/memfsd"
	"ulambda/perf"
)

const KV = "bin/kv"

type Elastic struct {
	mu    sync.Mutex
	cores map[string]bool
	done  uint32
	hz    int
	fsl   *fslib.FsLib
	pid   string
}

func MakeElastic(fsl *fslib.FsLib, pid string) *Elastic {
	e := &Elastic{}
	e.fsl = fsl
	e.hz = perf.Hz()
	e.pid = pid
	e.cores = map[string]bool{}
	linuxsched.ScanTopology()
	// Get the cores we can run on
	m, err := linuxsched.SchedGetAffinity(os.Getpid())
	if err != nil {
		log.Fatalf("Error getting affinity mask: %v", err)
	}
	for i := uint(0); i < linuxsched.NCores; i++ {
		if m.Test(i) {
			e.cores["cpu"+strconv.Itoa(int(i))] = true
		}
	}
	go e.monitorPID()
	return e
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

func runBalancer(fsl *fslib.FsLib, opcode, mfs string) {
	pid1 := spawnBalancer(fsl, opcode, mfs)
	ok, err := fsl.Wait(pid1)
	if string(ok) != "OK" || err != nil {
		log.Printf("runBalancer: ok %v err %v\n", string(ok), err)
	}
	log.Printf("balancer %v done\n", pid1)
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

// See if there is KV waiting to be run
func (e *Elastic) kvwaiting() bool {
	jobs, err := e.fsl.ReadWaitQ()
	if err != nil {
		log.Fatalf("grow: cannot read runq err %v\n", err)
	}
	for _, j := range jobs {
		log.Printf("job %v\n", j.Name)
		a, err := e.fsl.ReadWaitQJob(j.Name)
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

func (e *Elastic) grow() {
	log.Printf("grow\n")

	// A KV is waiting to run; maybe we maxed the resources.  Wait
	// and retry later
	if e.kvwaiting() {
		return
	}
	pid := spawnKV(e.fsl)
	for true {
		ok := e.fsl.HasBeenSpawned(pid)
		if ok {
			break
		}
	}
	log.Printf("kv running\n")
	runBalancer(e.fsl, "add", pid)
}

func (e *Elastic) shrink() {
	log.Printf("shrink: del %v\n", e.pid)
	runBalancer(e.fsl, "del", e.pid)
	err := e.fsl.Remove(memfsd.MEMFS + "/" + e.pid + "/")
	if err != nil {
		log.Printf("shrink: remove failed %v\n", err)
	}
	atomic.StoreUint32(&e.done, 1)
}

func (e *Elastic) monitorPID() {
	const (
		MAXLOAD float64 = 85.0
		MINLOAD float64 = 50.0
	)

	ms := 1000
	j := 1000 / e.hz
	var total0 uint64
	var total1 uint64
	pid := os.Getpid()
	total0 = perf.GetPIDSample(pid)
	for atomic.LoadUint32(&e.done) != 1 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		total1 = perf.GetPIDSample(pid)
		delta := total1 - total0
		util := 100.0 * float64(delta) / float64(ms/j)
		log.Printf("CPU delta: %v util %f\n", delta, util)
		if util >= MAXLOAD {
			e.grow()
		}
		if util < MINLOAD {
			e.shrink()
		}
		total0 = total1
	}
}

func (e *Elastic) Done() {
	atomic.StoreUint32(&e.done, 1)
}
