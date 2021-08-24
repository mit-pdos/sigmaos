package depproc

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/proc"
)

type Tstate struct {
	*DepProcCtl
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
	ts.DepProcCtl = MakeDepProcCtl(ts.FsLib, DEFAULT_JOB_ID)
	ts.t = t
	return ts
}

func makeTstateNoBoot(t *testing.T, s *kernel.System) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.s = s
	db.Name("sched_test")
	ts.FsLib = fslib.MakeFsLib("sched_test")
	ts.DepProcCtl = MakeDepProcCtl(ts.FsLib, DEFAULT_JOB_ID)
	return ts
}

func spawnSleeperlWithPid(t *testing.T, ts *Tstate, pid string) {
	spawnSleeperlWithPidDep(t, ts, pid, nil, nil)
}

// XXX FIX
func spawnMonitor(t *testing.T, ts *Tstate) {
	pid := "monitor-" + fslib.GenPid()
	a := MakeDepProc()
	a.Proc = &proc.Proc{pid, "bin/user/procd-monitor", "", []string{}, nil,
		proc.T_DEF, proc.C_DEF}
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DLPrintf("SCHEDD", "Spawn %v\n", a)
}

func spawnSleeperlWithDep(t *testing.T, ts *Tstate, startDep, exitDep map[string]bool) string {
	pid := fslib.GenPid()
	spawnSleeperlWithPidDep(t, ts, pid, startDep, exitDep)
	return pid
}

func spawnSleeperlWithPidDep(t *testing.T, ts *Tstate, pid string, startDep, exitDep map[string]bool) {
	a := MakeDepProc()
	a.Proc = &proc.Proc{pid, "bin/user/sleeperl", "", []string{"5s", "name/out_" + pid, ""}, nil, proc.T_DEF, proc.C_DEF}
	a.Dependencies.StartDep = startDep
	a.Dependencies.ExitDep = exitDep
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DLPrintf("SCHEDD", "Spawn %v\n", a)
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
	time.Sleep(6 * time.Second)

	checkSleeperlResult(t, ts, pid)

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

func TestExitDep(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()

	pid := spawnSleeperl(t, ts)

	pid2 := spawnSleeperlWithDep(t, ts, map[string]bool{}, map[string]bool{pid: false})

	// Make sure no-op waited for sleeperl lambda
	ts.WaitExit(pid2)
	end := time.Now()
	elapsed := end.Sub(start)
	assert.True(t, elapsed > 10*time.Second, "Didn't wait for exit dep for long enough")

	checkSleeperlResult(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

//func TestSwapExitDeps(t *testing.T) {
//	ts := makeTstate(t)
//
//	pid := spawnSleeperl(t, ts)
//
//	pid2 := spawnNoOp(t, ts, []string{pid})
//
//	start := time.Now()
//
//	// Sleep a bit
//	time.Sleep(4 * time.Second)
//
//	// Spawn a new sleeperl lambda
//	pid3 := spawnSleeperl(t, ts)
//
//	// Wait on the new sleeperl lambda instead of the old one
//	swaps := []string{pid, pid3}
//	db.DLPrintf("SCHEDD", "Swapping %v\n", swaps)
//	ts.SwapExitDependency(swaps)
//
//	ts.Wait(pid2)
//	end := time.Now()
//	elapsed := end.Sub(start)
//	assert.True(t, elapsed.Seconds() > 8.0, "Didn't wait for exit dep for long enough (%v)", elapsed.Seconds())
//
//	checkSleeperlResult(t, ts, pid)
//	checkSleeperlResult(t, ts, pid3)
//
//	ts.s.Shutdown(ts.FsLib)
//}

func TestStartDep(t *testing.T) {
	ts := makeTstate(t)

	// Generate a consumer & producer pid, make sure they dont' equal each other
	cons := fslib.GenPid()
	prod := fslib.GenPid()
	for cons == prod {
		prod = fslib.GenPid()
	}

	start := time.Now()

	// Spawn the producer first
	spawnSleeperlWithPidDep(t, ts, prod, map[string]bool{}, map[string]bool{})

	// Make sure the producer hasn't run yet...
	checkSleeperlResultFalse(t, ts, prod)

	// Spawn the consumer
	spawnSleeperlWithPidDep(t, ts, cons, map[string]bool{prod: false}, map[string]bool{})

	end := time.Now()

	err := ts.WaitExit(cons)
	assert.Nil(t, err, "WaitExit error")

	// Wait a bit
	assert.True(t, end.Sub(start) < 10*time.Second, "Start dep waited too long....")

	// Make sure they both ran
	checkSleeperlResult(t, ts, prod)
	checkSleeperlResult(t, ts, cons)

	ts.s.Shutdown(ts.FsLib)
}
