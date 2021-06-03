package kv

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	npo "ulambda/npobjsrv"
	"ulambda/perf"
)

const KV = "bin/kv"

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

// XXX monitor should take lock?
func (mo *Monitor) Work() {
	util := float64(0)
	n := 0
	sts, err := mo.ReadDir("name/memfsd")
	if err != nil {
		log.Printf("Readdir failed %v\n", err)
		os.Exit(1)
	}
	for _, st := range sts {
		kvd := "name/memfsd/" + st.Name + "/statsd"
		log.Printf("monitor: %v\n", kvd)
		sti := npo.StatInfo{}
		err := mo.ReadFileJson(kvd, &sti)
		if err != nil {
			log.Printf("ReadFileJson failed %v\n", err)
			os.Exit(1)
		}
		n += 1
		util += sti.Util
	}
	util = util / float64(n)
	log.Printf("monitor: avg util: %f\n", util)
	if util >= perf.MAXLOAD {
		mo.grow()
	}
}
