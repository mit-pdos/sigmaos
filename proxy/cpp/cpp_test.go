package cpp_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
	linuxsched "sigmaos/util/linux/sched"
)

func spawnSpinner(ts *test.RealmTstate) sp.Tpid {
	return spawnSpinnerMcpu(ts, 0)
}

func spawnSpinnerMcpu(ts *test.RealmTstate, mcpu proc.Tmcpu) sp.Tpid {
	pid := sp.GenPid("spinner")
	a := proc.NewProcPid(pid, "spinner", []string{"name/"})
	a.SetMcpu(mcpu)
	err := ts.Spawn(a)
	assert.Nil(ts.Ts.T, err, "Spawn")
	return pid
}

// Make a burst of LC procs
func burstSpawnSpinner(ts *test.RealmTstate, N uint) []*proc.Proc {
	ps := make([]*proc.Proc, 0, N)
	for i := uint(0); i < N; i++ {
		p := proc.NewProc("spinner", []string{"name/"})
		p.SetMcpu(1000)
		err := ts.Spawn(p)
		assert.Nil(ts.Ts.T, err, "Failed spawning some procs: %v", err)
		ps = append(ps, p)
	}
	return ps
}

func spawnSleeperWithPid(ts *test.RealmTstate, pid sp.Tpid) {
	spawnSleeperMcpu(ts, pid, 0, SLEEP_MSECS)
}

func spawnSleeper(ts *test.RealmTstate) sp.Tpid {
	pid := sp.GenPid("sleeper")
	spawnSleeperWithPid(ts, pid)
	return pid
}

func spawnSleeperMcpu(ts *test.RealmTstate, pid sp.Tpid, mcpu proc.Tmcpu, msecs int) {
	a := proc.NewProcPid(pid, "sleeper", []string{fmt.Sprintf("%dms", msecs), "name/"})
	a.SetMcpu(mcpu)
	err := ts.Spawn(a)
	assert.Nil(ts.Ts.T, err, "Spawn")
}

func spawnSpawner(ts *test.RealmTstate, wait bool, childPid sp.Tpid, msecs int, em *crash.TeventMap) sp.Tpid {
	p := proc.NewProc("spawner", []string{strconv.FormatBool(wait), childPid.String(), "sleeper", fmt.Sprintf("%dms", msecs), "name/"})
	err := em.AppendEnv(p)
	assert.Nil(ts.Ts.T, err)
	err = ts.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Spawn")
	return p.GetPid()
}

func checkSleeperResult(ts *test.RealmTstate, pid sp.Tpid) bool {
	res := true
	b, err := ts.GetFile("name/" + pid.String() + "_out")
	res = assert.Nil(ts.Ts.T, err, "GetFile err: %v", err) && res
	res = assert.Equal(ts.Ts.T, "hello", string(b), "Output") && res

	return res
}

func checkSleeperResultFalse(ts *test.RealmTstate, pid sp.Tpid) {
	b, err := ts.GetFile("name/" + pid.String() + "_out")
	assert.NotNil(ts.Ts.T, err, "GetFile")
	assert.NotEqual(ts.Ts.T, string(b), "hello", "Output")
}

func cleanSleeperResult(ts *test.RealmTstate, pid sp.Tpid) {
	ts.SigmaClnt.Remove("name/" + pid.String() + "_out")
}

func spawnWaitSleeper(ts *test.RealmTstate, kernels []string) *proc.Proc {
	p := proc.NewProc("spawn-latency-cpp", []string{})
	err := ts.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Spawn")

	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(ts.Ts.T, err, "WaitExit error")
	assert.True(ts.Ts.T, status != nil && status.IsStatusOK(), "Exit status wrong: %v", status)
	return p
}

func TestCompile(t *testing.T) {
}

func TestSpawnWaitExit(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	runSpawnLatency(mrts.GetRealm(REALM1), nil)
}
