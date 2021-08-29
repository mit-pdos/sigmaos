package baseproc_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/proc"
	"ulambda/procinit"
)

type Tstate struct {
	proc.ProcCtl
	*fslib.FsLib
	t *testing.T
	s *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	procinit.SetProcLayers(map[string]bool{procinit.BASEPROC: true})

	bin := ".."
	s, err := kernel.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	db.Name("sched_test")

	ts.FsLib = fslib.MakeFsLib("sched_test")
	ts.ProcCtl = procinit.MakeProcCtl(ts.FsLib, procinit.GetProcLayersMap())
	ts.t = t
	return ts
}

func makeTstateNoBoot(t *testing.T, s *kernel.System) *Tstate {
	ts := &Tstate{}
	procinit.SetProcLayers(map[string]bool{procinit.BASEPROC: true})
	ts.t = t
	ts.s = s
	db.Name("sched_test")
	ts.FsLib = fslib.MakeFsLib("sched_test")
	ts.ProcCtl = procinit.MakeProcCtl(ts.FsLib, procinit.GetProcLayersMap())
	return ts
}

func spawnSleeperlWithPid(t *testing.T, ts *Tstate, pid string) {
	a := &proc.Proc{pid, "bin/user/sleeperl", "",
		[]string{"5s", "name/out_" + pid, ""},
		[]string{procinit.GetProcLayersString()},
		proc.T_DEF, proc.C_DEF,
	}
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

func TestWaitExit(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()

	pid := spawnSleeperl(t, ts)
	err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")

	end := time.Now()

	assert.True(t, end.Sub(start) > 5*time.Second)

	checkSleeperlResult(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

func TestWaitStart(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()

	pid := spawnSleeperl(t, ts)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")

	end := time.Now()

	assert.True(t, end.Sub(start) < 5*time.Second, "WaitStart waited too long")

	// Make sure the lambda hasn't finished yet...
	checkSleeperlResultFalse(t, ts, pid)

	ts.WaitExit(pid)

	checkSleeperlResult(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

// Should exit immediately
func TestWaitNonexistentLambda(t *testing.T) {
	ts := makeTstate(t)

	ch := make(chan bool)

	pid := fslib.GenPid()
	go func() {
		ts.WaitExit(pid)
		ch <- true
	}()

	done := <-ch
	assert.True(t, done, "Nonexistent lambda")

	close(ch)

	ts.s.Shutdown(ts.FsLib)
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
		pid := fslib.GenPid()
		_, alreadySpawned := pids[pid]
		for alreadySpawned {
			pid = fslib.GenPid()
			_, alreadySpawned = pids[pid]
		}
		pids[pid] = i
		newts := makeTstateNoBoot(t, ts.s)
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

	ts.s.Shutdown(ts.FsLib)
}

func (ts *Tstate) evict(pid string) {
	time.Sleep(1 * time.Second)
	err := ts.Evict(pid)
	assert.Nil(ts.t, err, "evict")
}

func TestEvict(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()
	pid := spawnSleeperl(t, ts)

	go ts.evict(pid)

	err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	end := time.Now()

	assert.True(t, end.Sub(start) < 3*time.Second, "Didn't evict early enough.")
	assert.True(t, end.Sub(start) > 1*time.Second, "Evicted too early")

	// Make sure the lambda didn't finish
	checkSleeperlResultFalse(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}
