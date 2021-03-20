package schedd

import (
	//	"sync"
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

func makeTstateNoBoot(t *testing.T, s *fslib.System) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.s = s
	db.Name("sched_test")
	ts.FsLib = fslib.MakeFsLib("sched_test")
	return ts
}

func spawnSchedlWithPid(t *testing.T, ts *Tstate, pid string) {
	a := &fslib.Attr{pid, "bin/schedl", "", []string{"name/out_" + pid, ""}, nil, nil, nil}
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DLPrintf("SCHEDD", "Spawn %v\n", a)
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

	db.DLPrintf("SCHEDD", "SpawnNoOp %v\n", pid)
	return pid
}

func TestWait(t *testing.T) {
	ts := makeTstate(t)

	pid := spawnSchedl(t, ts)
	ts.Wait(pid)

	checkSchedlResult(t, ts, pid)

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

	for i := 0; i < 30; i++ {
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

// XXX Wait signal gets dropped
// Should exit immediately
//func TestNoOpLambdaImmediateExit(t *testing.T) {
//	ts := makeTstate(t)
//
//	pid := spawnNoOp(t, ts, []string{})
//
//	ch := make(chan bool)
//
//	go func() {
//		log.Printf("pre wait")
//		ts.Wait(pid)
//		log.Printf("Post wait %v", pid)
//		ch <- true
//		log.Printf("Post send")
//	}()
//
//	for i := 0; i < 500; i++ {
//		log.Printf("About to test channel")
//		select {
//		case done := <-ch:
//			log.Printf("done waiting")
//			assert.True(t, done, "No-op lambda")
//			break
//		default:
//			log.Printf("waiting longer")
//			time.Sleep(10 * time.Millisecond)
//		}
//	}
//
//	close(ch)
//
//	ts.s.Shutdown(ts.FsLib)
//}

func TestExitDep(t *testing.T) {
	ts := makeTstate(t)

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

	pid := spawnSchedl(t, ts)

	pid2 := spawnNoOp(t, ts, []string{pid})

	start := time.Now()

	// Sleep a bit
	time.Sleep(4 * time.Second)

	// Spawn a new schedl lambda
	pid3 := spawnSchedl(t, ts)

	// Wait on the new schedl lambda instead of the old one
	swaps := []string{pid, pid3}
	db.DLPrintf("SCHEDD", "Swapping %v\n", swaps)
	ts.SwapExitDependency(swaps)

	ts.Wait(pid2)
	end := time.Now()
	elapsed := end.Sub(start)
	assert.True(t, elapsed.Seconds() > 8.0, "Didn't wait for exit dep for long enough (%v)", elapsed.Seconds())

	checkSchedlResult(t, ts, pid)
	checkSchedlResult(t, ts, pid3)

	ts.s.Shutdown(ts.FsLib)
}

// XXX Wait signal gets dropped
// Spawn a bunch of lambdas concurrently, then wait for all of them & check
// their result
//func TestConcurrentLambdas(t *testing.T) {
//	ts := makeTstate(t)
//
//
//	nLambdas := 27
//	pids := map[string]int{}
//
//	// Make a bunch of fslibs to avoid concurrency issues
//	tses := []*Tstate{}
//
//	for j := 0; j < nLambdas; j++ {
//	}
//
//	var barrier sync.WaitGroup
//	barrier.Add(nLambdas)
//	var started sync.WaitGroup
//	started.Add(nLambdas)
//	var done sync.WaitGroup
//	done.Add(nLambdas)
//
//	for i := 0; i < nLambdas; i++ {
//		pid := fslib.GenPid()
//		_, alreadySpawned := pids[pid]
//		for alreadySpawned {
//			pid = fslib.GenPid()
//			_, alreadySpawned = pids[pid]
//		}
//		pids[pid] = i
//		newts := makeTstateNoBoot(t, ts.s)
//		tses = append(tses, newts)
//		go func(pid string, started *sync.WaitGroup, i int) {
//			barrier.Done()
//			barrier.Wait()
//			spawnSchedlWithPid(t, tses[i], pid)
//			log.Printf("Starting with pid %v\n", pid)
//			started.Done()
//		}(pid, &started, i)
//	}
//
//	started.Wait()
//
//	//	time.Sleep(2 * time.Second)
//
//	for pid, i := range pids {
//		_ = i
//		go func(pid string, done *sync.WaitGroup, i int) {
//			defer done.Done()
//			log.Printf("Going to wait for and check %v\n", pid)
//			ts.Wait(pid)
//			checkSchedlResult(t, tses[i], pid)
//			log.Printf("Done waiting for %v\n", pid)
//		}(pid, &done, i)
//	}
//
//	done.Wait()
//
//	ts.s.Shutdown(ts.FsLib)
//}
