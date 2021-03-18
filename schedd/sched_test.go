package schedd

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/debug"
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

	ts.FsLib = fslib.MakeFsLib("schedl")
	ts.t = t

	return ts
}

func spawnSchedlWithPid(t *testing.T, ts *Tstate, pid string) {
	a := &fslib.Attr{pid, "bin/schedl", "", []string{"name/out_" + pid, ""}, nil, nil, nil}
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	log.Printf("Spawn %v\n", a)
}

func spawnSchedl(t *testing.T, ts *Tstate) string {
	pid := fslib.GenPid()
	spawnSchedlWithPid(t, ts, pid)
	return pid
}

func checkSchedlResult(t *testing.T, ts *Tstate, pid string) {
	b, err := ts.ReadFile("name/out_" + pid)
	assert.Nil(t, err, "ReadFile")
	assert.Equal(t, string(b), "hello", "Output")
}

func spawnNoOp(t *testing.T, ts *Tstate, deps []string) string {
	pid := fslib.GenPid()
	err := ts.SpawnNoOp(pid, deps)
	assert.Nil(t, err, "SpawnNoOp")

	log.Printf("SpawnNoOp\n")
	return pid
}

func TestWait(t *testing.T) {
	ts := makeTstate(t)

	debug.SetDebug(false)

	pid := spawnSchedl(t, ts)
	ts.Wait(pid)

	checkSchedlResult(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

// Should exit immediately
func TestWaitNonexistentLambda(t *testing.T) {
	ts := makeTstate(t)

	debug.SetDebug(false)

	ch := make(chan bool)

	pid := fslib.GenPid()
	go func() {
		ts.Wait(pid)
		ch <- true
	}()

	for i := 0; i < 30; i++ {
		select {
		case done := <-ch:
			assert.True(t, done, "Nonexistent lambda")
			break
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	log.Printf("Wait on nonexistent lambda\n")

	close(ch)

	ts.s.Shutdown(ts.FsLib)
}

// Should exit immediately
func TestNoOpLambdaImmediateExit(t *testing.T) {
	ts := makeTstate(t)

	debug.SetDebug(false)

	pid := spawnNoOp(t, ts, []string{})

	ch := make(chan bool)

	go func() {
		ts.Wait(pid)
		ch <- true
	}()

	for i := 0; i < 30; i++ {
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

func TestExitDep(t *testing.T) {
	ts := makeTstate(t)

	debug.SetDebug(false)

	pid := spawnSchedl(t, ts)

	pid2 := spawnNoOp(t, ts, []string{pid})

	// Make sure no-op waited for schedl lambda
	start := time.Now()
	ts.Wait(pid2)
	end := time.Now()
	elapsed := end.Sub(start)
	assert.True(t, elapsed.Seconds() > 4.0, "Didn't wait for exit dep for long enough")

	checkSchedlResult(t, ts, pid)

	ts.s.Shutdown(ts.FsLib)
}

func TestSwapExitDeps(t *testing.T) {
	ts := makeTstate(t)

	debug.SetDebug(false)

	pid := spawnSchedl(t, ts)

	pid2 := spawnNoOp(t, ts, []string{pid})

	start := time.Now()

	// Sleep a bit
	time.Sleep(4 * time.Second)

	// Spawn a new schedl lambda
	pid3 := spawnSchedl(t, ts)

	// Wait on the new schedl lambda instead of the old one
	swaps := []string{pid, pid3}
	log.Printf("Swapping %v\n", swaps)
	ts.SwapExitDependency(swaps)

	ts.Wait(pid2)
	end := time.Now()
	elapsed := end.Sub(start)
	assert.True(t, elapsed.Seconds() > 8.0, "Didn't wait for exit dep for long enough (%v)", elapsed.Seconds())

	checkSchedlResult(t, ts, pid)
	checkSchedlResult(t, ts, pid3)

	ts.s.Shutdown(ts.FsLib)
}

func TestConcurrentLambdas(t *testing.T) {
	ts := makeTstate(t)

	debug.SetDebug(false)

	nLambdas := 5
	pids := []string{}

	var start sync.WaitGroup
	start.Add(nLambdas)
	var done sync.WaitGroup
	done.Add(nLambdas)

	for i := 0; i < nLambdas; i++ {
		pid := fslib.GenPid()
		pids = append(pids, pid)
		go func(pid string, start *sync.WaitGroup) {
			start.Done()
			start.Wait()
			spawnSchedlWithPid(t, ts, pid)
		}(pid, &start)
	}

	for _, pid := range pids {
		go func(pid string, done *sync.WaitGroup) {
			defer done.Done()
			ts.Wait(pid)
			checkSchedlResult(t, ts, pid)
		}(pid, &done)
	}

	done.Wait()

	ts.s.Shutdown(ts.FsLib)
}
