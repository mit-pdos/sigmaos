package locald

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
)

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *fslib.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	db.Name("sched_test")

	ts.FsLib = fslib.MakeFsLib("sched_test")
	ts.t = t
	return ts
}

func makeTstateOneLocald(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := fslib.BootMin(bin)
	if err != nil {
		t.Fatalf("BootMin %v\n", err)
	}
	ts.s = s
	db.Name("sched_test")
	err = ts.s.BootLocald(bin)
	if err != nil {
		t.Fatalf("BootLocald %v\n", err)
	}

	ts.FsLib = fslib.MakeFsLib("sched_test")
	ts.t = t
	return ts
}

func makeTstateNoBoot(t *testing.T, s *fslib.System) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.s = s
	db.Name("sched_test")
	ts.FsLib = fslib.MakeFsLib("sched_test")
	return ts
}

func spawnSleeperlWithPid(t *testing.T, ts *Tstate, pid string) {
	spawnSleeperlWithPidTimer(t, ts, pid, 0)
}

func spawnMonitor(t *testing.T, ts *Tstate) {
	pid := "monitor-" + fslib.GenPid()
	a := &fslib.Attr{pid, "bin/locald-monitor", "", []string{}, nil, nil, nil, 1}
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DLPrintf("SCHEDD", "Spawn %v\n", a)
}

func spawnSleeperlWithPidTimer(t *testing.T, ts *Tstate, pid string, timer uint32) {
	a := &fslib.Attr{pid, "bin/sleeperl", "", []string{"name/out_" + pid, ""}, nil, nil, nil, timer}
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DLPrintf("SCHEDD", "Spawn %v\n", a)
}

func spawnSleeperl(t *testing.T, ts *Tstate) string {
	pid := fslib.GenPid()
	spawnSleeperlWithPid(t, ts, pid)
	return pid
}

func spawnSleeperlWithTimer(t *testing.T, ts *Tstate, timer uint32) string {
	pid := fslib.GenPid()
	spawnSleeperlWithPidTimer(t, ts, pid, timer)
	return pid
}

func checkSleeperlResult(t *testing.T, ts *Tstate, pid string) {
	b, err := ts.ReadFile("name/out_" + pid)
	assert.Nil(t, err, "ReadFile")
	assert.Equal(t, string(b), "hello", "Output")
}

func spawnNoOp(t *testing.T, ts *Tstate, deps []string) string {
	pid := fslib.GenPid()
	err := ts.SpawnNoOp(pid, deps)
	assert.Nil(t, err, "SpawnNoOp")

	db.DLPrintf("SCHEDD", "SpawnNoOp %v\n", pid)
	return pid
}

func TestHelloWorld(t *testing.T) {
	ts := makeTstate(t)

	pid := spawnSleeperl(t, ts)
	time.Sleep(10 * time.Second)

	checkSleeperlResult(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

func TestWait(t *testing.T) {
	ts := makeTstate(t)

	pid := spawnSleeperl(t, ts)
	ts.Wait(pid)

	checkSleeperlResult(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

// Should exit immediately
func TestWaitNonexistentLambda(t *testing.T) {
	ts := makeTstate(t)

	ch := make(chan bool)

	pid := fslib.GenPid()
	go func() {
		ts.Wait(pid)
		ch <- true
	}()

	for i := 0; i < 50; i++ {
		select {
		case done := <-ch:
			assert.True(t, done, "Nonexistent lambda")
			break
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	db.DLPrintf("SCHEDD", "Wait on nonexistent lambda\n")

	close(ch)

	ts.s.Shutdown(ts.FsLib)
}

// Should exit immediately
func TestNoOpLambdaImmediateExit(t *testing.T) {
	ts := makeTstate(t)

	pid := spawnNoOp(t, ts, []string{})

	ch := make(chan bool)

	go func() {
		ts.Wait(pid)
		ch <- true
	}()

	for i := 0; i < 500; i++ {
		select {
		case done := <-ch:
			assert.True(t, done, "No-op lambda")
			break
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	close(ch)

	ts.s.Shutdown(ts.FsLib)
}

// Should start after 5 secs
func TestTimerLambda(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()
	pid := spawnSleeperlWithTimer(t, ts, 5)
	ts.Wait(pid)
	end := time.Now()
	elapsed := end.Sub(start)
	assert.True(t, elapsed.Seconds() > 9.0, "Didn't wait for timer for long enough (%v)", elapsed.Seconds())

	checkSleeperlResult(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

// Start a locald, crash it, start a new one, and make sure it reruns lambdas.
func TestCrashLocald(t *testing.T) {
	ts := makeTstateOneLocald(t)

	ch := make(chan bool)
	spawnMonitor(t, ts)
	go func() {
		start := time.Now()
		pid := spawnSleeperlWithTimer(t, ts, 5)
		ts.Wait(pid)
		end := time.Now()
		elapsed := end.Sub(start)
		assert.True(t, elapsed.Seconds() > 9.0, "Didn't wait for respawn after locald crash (%v)", elapsed.Seconds())
		checkSleeperlResult(t, ts, pid)
		ch <- true
	}()

	// Wait for a bit
	time.Sleep(1 * time.Second)

	// Kill the locald instance
	ts.s.Kill(fslib.LOCALD)

	// Wait for a bit
	time.Sleep(10 * time.Second)

	//	ts.SignalNewJob()

	err := ts.s.BootLocald("..")
	if err != nil {
		t.Fatalf("BootLocald %v\n", err)
	}

	<-ch
	ts.s.Shutdown(ts.FsLib)
}

func TestExitDep(t *testing.T) {
	ts := makeTstate(t)

	pid := spawnSleeperl(t, ts)

	pid2 := spawnNoOp(t, ts, []string{pid})

	// Make sure no-op waited for sleeperl lambda
	start := time.Now()
	ts.Wait(pid2)
	end := time.Now()
	elapsed := end.Sub(start)
	assert.True(t, elapsed.Seconds() > 4.0, "Didn't wait for exit dep for long enough")

	checkSleeperlResult(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

func TestSwapExitDeps(t *testing.T) {
	ts := makeTstate(t)

	pid := spawnSleeperl(t, ts)

	pid2 := spawnNoOp(t, ts, []string{pid})

	start := time.Now()

	// Sleep a bit
	time.Sleep(4 * time.Second)

	// Spawn a new sleeperl lambda
	pid3 := spawnSleeperl(t, ts)

	// Wait on the new sleeperl lambda instead of the old one
	swaps := []string{pid, pid3}
	db.DLPrintf("SCHEDD", "Swapping %v\n", swaps)
	ts.SwapExitDependency(swaps)

	ts.Wait(pid2)
	end := time.Now()
	elapsed := end.Sub(start)
	assert.True(t, elapsed.Seconds() > 8.0, "Didn't wait for exit dep for long enough (%v)", elapsed.Seconds())

	checkSleeperlResult(t, ts, pid)
	checkSleeperlResult(t, ts, pid3)

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
			ts.Wait(pid)
			checkSleeperlResult(t, tses[i], pid)
		}(pid, &done, i)
	}

	done.Wait()

	ts.s.Shutdown(ts.FsLib)
}
