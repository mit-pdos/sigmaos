package idemproc

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/proc"
)

type Tstate struct {
	*IdemProcCtl
	*fslib.FsLib
	t *testing.T
	s *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := kernel.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	db.Name("sched_test")

	ts.FsLib = fslib.MakeFsLib("sched_test")
	ts.IdemProcCtl = MakeIdemProcCtl(ts.FsLib)
	ts.IdemProcCtl.Init()
	ts.t = t
	return ts
}

func makeTstateNoBoot(t *testing.T, s *kernel.System) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.s = s
	db.Name("sched_test")
	ts.FsLib = fslib.MakeFsLib("sched_test")
	ts.IdemProcCtl = MakeIdemProcCtl(ts.FsLib)
	ts.IdemProcCtl.Init()
	return ts
}

func spawnMonitor(t *testing.T, ts *Tstate, pid string) {
	p := &IdemProc{}
	p.Proc = &proc.Proc{pid, "bin/user/idemproc-monitor", "", []string{}, nil, proc.T_DEF, proc.C_DEF}
	err := ts.Spawn(p)
	assert.Nil(t, err, "Monitor spawn")
}

func spawnSleeperlWithPid(t *testing.T, ts *Tstate, pid string) {
	p := &IdemProc{}
	p.Proc = &proc.Proc{pid, "bin/user/sleeperl", "", []string{"5s", "name/out_" + pid, ""}, nil, proc.T_DEF, proc.C_DEF}
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
}

func spawnSleeperl(t *testing.T, ts *Tstate) string {
	pid := fslib.GenPid()
	spawnSleeperlWithPid(t, ts, pid)
	return pid
}

func checkSleeperlResult(t *testing.T, ts *Tstate, pid string) bool {
	res := true
	b, err := ts.ReadFile("name/out_" + pid)
	res = assert.Nil(t, err, "ReadFile") && res
	res = assert.Equal(t, string(b), "hello", "Output") && res
	return res
}

func checkSleeperlResultFalse(t *testing.T, ts *Tstate, pid string) {
	b, err := ts.ReadFile("name/out_" + pid)
	assert.NotNil(t, err, "ReadFile")
	assert.NotEqual(t, string(b), "hello", "Output")
}

func TestHelloWorld(t *testing.T) {
	ts := makeTstate(t)

	pid := spawnSleeperl(t, ts)
	time.Sleep(3 * time.Second)

	ts.s.KillOne(kernel.PROCD)

	time.Sleep(3 * time.Second)

	checkSleeperlResultFalse(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

func TestCrashProcd(t *testing.T) {
	ts := makeTstate(t)

	ts.s.BootProcd("..")

	N_MON := 5
	N_SLEEP := 5

	monPids := []string{}
	for i := 0; i < N_MON; i++ {
		pid := fslib.GenPid()
		spawnMonitor(t, ts, pid)
		monPids = append(monPids, pid)
	}

	time.Sleep(time.Second * 3)

	// Spawn some sleepers
	sleeperPids := []string{}
	for i := 0; i < N_SLEEP; i++ {
		pid := fslib.GenPid()
		spawnSleeperlWithPid(t, ts, pid)
		sleeperPids = append(sleeperPids, pid)
	}

	time.Sleep(time.Second * 1)

	ts.s.KillOne(kernel.PROCD)
	log.Printf("Killed a procd")

	time.Sleep(time.Second * 10)

	for _, pid := range sleeperPids {
		checkSleeperlResult(t, ts, pid)
	}

	log.Printf("Start evictions")
	for _, pid := range monPids {
		ts.Evict(pid)
	}
	log.Printf("Finish evictions")

	ts.s.Shutdown(ts.FsLib)
}

// Start a procd, crash it, start a new one, and make sure it reruns lambdas.
//func TestCrashProcd(t *testing.T) {
//	ts := makeTstateOneProcd(t)
//
//	ch := make(chan bool)
//	spawnMonitor(t, ts)
//	go func() {
//		start := time.Now()
//		pid := spawnSleeperlWithTimer(t, ts, 5)
//		ts.Wait(pid)
//		end := time.Now()
//		elapsed := end.Sub(start)
//		assert.True(t, elapsed.Seconds() > 9.0, "Didn't wait for respawn after procd crash (%v)", elapsed.Seconds())
//		checkSleeperlResult(t, ts, pid)
//		ch <- true
//	}()
//
//	// Wait for a bit
//	time.Sleep(1 * time.Second)
//
//	// Kill the procd instance
//	ts.s.Kill(fslib.PROCD)
//
//	// Wait for a bit
//	time.Sleep(10 * time.Second)
//
//	//	ts.SignalNewJob()
//
//	err := ts.s.BootProcd("..")
//	if err != nil {
//		t.Fatalf("BootProcd %v\n", err)
//	}
//
//	<-ch
//	ts.s.Shutdown(ts.FsLib)
//}
