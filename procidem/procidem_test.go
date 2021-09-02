package procidem_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/proc"
	"ulambda/procidem"
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

	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true, procinit.PROCIDEM: true})

	bin := ".."
	s := kernel.MakeSystem(bin)
	err := s.Boot()
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

func spawnMonitor(t *testing.T, ts *Tstate, pid string) {
	p := &procidem.ProcIdem{}
	p.Proc = &proc.Proc{pid, "bin/user/procd-monitor", "",
		[]string{},
		[]string{procinit.GetProcLayersString()},
		proc.T_DEF, proc.C_DEF,
	}
	err := ts.Spawn(p)
	assert.Nil(t, err, "Monitor spawn")
}

func spawnSleeperlWithPid(t *testing.T, ts *Tstate, pid string) {
	p := &procidem.ProcIdem{}
	p.Proc = &proc.Proc{pid, "bin/user/sleeperl", "",
		[]string{"5s", "name/out_" + pid, ""},
		[]string{procinit.GetProcLayersString()},
		proc.T_DEF, proc.C_DEF,
	}
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
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
	time.Sleep(3 * time.Second)

	ts.s.KillOne(kernel.PROCD)

	time.Sleep(3 * time.Second)

	checkSleeperlResultFalse(t, ts, pid)

	ts.s.Shutdown()
}

func TestCrashProcd(t *testing.T) {
	ts := makeTstate(t)

	ts.s.BootProcd("..")

	N_MON := 5
	N_SLEEP := 5

	monPids := []string{}
	for i := 0; i < N_MON; i++ {
		pid := proc.GenPid()
		spawnMonitor(t, ts, pid)
		monPids = append(monPids, pid)
	}

	time.Sleep(time.Second * 3)

	// Spawn some sleepers
	sleeperPids := []string{}
	for i := 0; i < N_SLEEP; i++ {
		pid := proc.GenPid()
		spawnSleeperlWithPid(t, ts, pid)
		sleeperPids = append(sleeperPids, pid)
	}

	time.Sleep(time.Second * 1)

	ts.s.KillOne(kernel.PROCD)

	for _, pid := range sleeperPids {
		ts.WaitExit(pid)
	}

	for _, pid := range sleeperPids {
		checkSleeperlResult(t, ts, pid)
	}

	for _, pid := range monPids {
		ts.Evict(pid)
	}

	ts.s.Shutdown()
}
