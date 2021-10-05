package procbase_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/realm"
)

const (
	SLEEP_MSECS = 2000
)

type Tstate struct {
	proc.ProcClnt
	*fslib.FsLib
	t   *testing.T
	e   *realm.TestEnv
	cfg *realm.RealmConfig
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})

	bin := ".."
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg

	db.Name("sched_test")

	ts.FsLib = fslib.MakeFsLibAddr("sched_test", ts.cfg.NamedAddr)
	ts.ProcClnt = procinit.MakeProcClnt(ts.FsLib, procinit.GetProcLayersMap())
	ts.t = t
	return ts
}

func (ts *Tstate) procd(t *testing.T) string {
	st, err := ts.ReadDir("name/procd")
	assert.Nil(t, err, "Readdir")
	return st[0].Name
}

func makeTstateNoBoot(t *testing.T, cfg *realm.RealmConfig, e *realm.TestEnv) *Tstate {
	ts := &Tstate{}
	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})
	ts.t = t
	ts.e = e
	ts.cfg = cfg
	db.Name("sched_test")
	ts.FsLib = fslib.MakeFsLibAddr("sched_test", ts.cfg.NamedAddr)
	ts.ProcClnt = procinit.MakeProcClnt(ts.FsLib, procinit.GetProcLayersMap())
	return ts
}

func spawnSleeperlWithPid(t *testing.T, ts *Tstate, pid string) {
	a := &proc.Proc{pid, "bin/user/sleeperl", "",
		[]string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/out_" + pid},
		[]string{procinit.GetProcLayersString()},
		proc.T_DEF, proc.C_DEF,
	}
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

	time.Sleep(SLEEP_MSECS * 1.25 * time.Millisecond)

	checkSleeperlResult(t, ts, pid)

	st, err := ts.ReadDir("name/procd/" + ts.procd(t) + "/")
	assert.Nil(t, err, "Readdir")
	assert.Equal(t, 0, len(st), "readdir")

	ts.e.Shutdown()
}

func TestWaitExit(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()

	pid := spawnSleeperl(t, ts)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.Equal(t, status, "OK", "Exit status wrong")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperlResult(t, ts, pid)

	ts.e.Shutdown()
}

func TestWaitExitParentRetStat(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()

	pid := spawnSleeperl(t, ts)
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.Equal(t, "OK", status, "Exit status wrong")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperlResult(t, ts, pid)

	ts.e.Shutdown()
}

func TestWaitStart(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()

	pid := spawnSleeperl(t, ts)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")

	end := time.Now()

	assert.True(t, end.Sub(start) < SLEEP_MSECS*time.Millisecond, "WaitStart waited too long")

	// Check if proc exists
	st, err := ts.ReadDir("name/procd/" + ts.procd(t) + "/")
	assert.Nil(t, err, "Readdir")
	assert.Equal(t, pid, st[0].Name, "pid")

	// Make sure the lambda hasn't finished yet...
	checkSleeperlResultFalse(t, ts, pid)

	ts.WaitExit(pid)

	checkSleeperlResult(t, ts, pid)

	ts.e.Shutdown()
}

// Should exit immediately
func TestWaitNonexistentLambda(t *testing.T) {
	ts := makeTstate(t)

	ch := make(chan bool)

	pid := proc.GenPid()
	go func() {
		ts.WaitExit(pid)
		ch <- true
	}()

	done := <-ch
	assert.True(t, done, "Nonexistent lambda")

	close(ch)

	ts.e.Shutdown()
}

// Spawn a bunch of lambdas concurrently, then wait for all of them & check
// their result
func TestConcurrentLambdas(t *testing.T) {
	ts := makeTstate(t)

	nLambdas := 27
	pids := map[string]int{}

	// Make a bunch of fslibs to avoid concurrency issues
	tses := []*Tstate{}

	for j := 0; j < nLambdas; j++ {
	}

	var barrier sync.WaitGroup
	barrier.Add(nLambdas)
	var started sync.WaitGroup
	started.Add(nLambdas)
	var done sync.WaitGroup
	done.Add(nLambdas)

	for i := 0; i < nLambdas; i++ {
		pid := proc.GenPid()
		_, alreadySpawned := pids[pid]
		for alreadySpawned {
			pid = proc.GenPid()
			_, alreadySpawned = pids[pid]
		}
		pids[pid] = i
		newts := makeTstateNoBoot(t, ts.cfg, ts.e)
		tses = append(tses, newts)
		go func(pid string, started *sync.WaitGroup, i int) {
			barrier.Done()
			barrier.Wait()
			spawnSleeperlWithPid(t, tses[i], pid)
			started.Done()
		}(pid, &started, i)
	}

	started.Wait()

	for pid, i := range pids {
		_ = i
		go func(pid string, done *sync.WaitGroup, i int) {
			defer done.Done()
			ts.WaitExit(pid)
			checkSleeperlResult(t, tses[i], pid)
		}(pid, &done, i)
	}

	done.Wait()

	ts.e.Shutdown()
}

func (ts *Tstate) evict(pid string) {
	time.Sleep(SLEEP_MSECS / 2 * time.Millisecond)
	err := ts.Evict(pid)
	assert.Nil(ts.t, err, "evict")
}

func TestEvict(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()
	pid := spawnSleeperl(t, ts)

	go ts.evict(pid)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.Equal(t, "EVICTED", status, "WaitExit status")
	end := time.Now()

	assert.True(t, end.Sub(start) < SLEEP_MSECS*time.Millisecond, "Didn't evict early enough.")
	assert.True(t, end.Sub(start) > (SLEEP_MSECS/2)*time.Millisecond, "Evicted too early")

	// Make sure the lambda didn't finish
	checkSleeperlResultFalse(t, ts, pid)

	ts.e.Shutdown()
}
