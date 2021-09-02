package procdep_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/procdep"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/proc"
	"ulambda/procinit"
)

type Tstate struct {
	proc.ProcClnt
	*fslib.FsLib
	t *testing.T
	s *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true, procinit.PROCDEP: true})

	bin := ".."
	s, err := kernel.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	db.Name("sched_test")

	ts.FsLib = fslib.MakeFsLib("sched_test")
	ts.ProcClnt = procinit.MakeProcClnt(ts.FsLib, procinit.GetProcLayersMap())
	ts.t = t
	return ts
}

func spawnSleeperlWithPid(t *testing.T, ts *Tstate, pid string) {
	spawnSleeperlWithPidDep(t, ts, pid, nil, nil)
}

func spawnSleeperlWithDep(t *testing.T, ts *Tstate, startDep, exitDep map[string]bool) string {
	pid := proc.GenPid()
	spawnSleeperlWithPidDep(t, ts, pid, startDep, exitDep)
	return pid
}

func spawnSleeperlWithPidDep(t *testing.T, ts *Tstate, pid string, startDep, exitDep map[string]bool) {
	a := procdep.MakeProcDep()
	a.Proc = &proc.Proc{pid, "bin/user/sleeperl", "",
		[]string{"5s", "name/out_" + pid, ""},
		[]string{procinit.GetProcLayersString()},
		proc.T_DEF, proc.C_DEF,
	}
	a.Dependencies.StartDep = startDep
	a.Dependencies.ExitDep = exitDep
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DLPrintf("SCHEDD", "Spawn %v\n", a)
}

func spawnSleeperl(t *testing.T, ts *Tstate) string {
	pid := proc.GenPid()
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

func TestStartDep(t *testing.T) {
	ts := makeTstate(t)

	// Generate a consumer & producer pid, make sure they dont' equal each other
	cons := proc.GenPid()
	prod := proc.GenPid()
	for cons == prod {
		prod = proc.GenPid()
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
