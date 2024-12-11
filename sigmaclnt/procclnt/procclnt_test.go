package procclnt_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/util/crash"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	linuxsched "sigmaos/util/linux/sched"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SLEEP_MSECS = 2000
	CRASH_MSECS = 5
	NTRIALS     = "3001"
	N_NODES     = 2
)

const program = "procclnt_test"

func msched(ts *test.Tstate) string {
	st, err := ts.GetDir(sp.MSCHED)
	assert.Nil(ts.T, err, "Readdir")
	return st[0].Name
}

func spawnSpinner(t *testing.T, ts *test.Tstate) sp.Tpid {
	return spawnSpinnerMcpu(ts, 0)
}

func spawnSpinnerMcpu(ts *test.Tstate, mcpu proc.Tmcpu) sp.Tpid {
	pid := sp.GenPid("spinner")
	a := proc.NewProcPid(pid, "spinner", []string{"name/"})
	a.SetMcpu(mcpu)
	err := ts.Spawn(a)
	assert.Nil(ts.T, err, "Spawn")
	return pid
}

// Make a burst of LC procs
func burstSpawnSpinner(t *testing.T, ts *test.Tstate, N uint) []*proc.Proc {
	ps := make([]*proc.Proc, 0, N)
	for i := uint(0); i < N; i++ {
		p := proc.NewProc("spinner", []string{"name/"})
		p.SetMcpu(1000)
		err := ts.Spawn(p)
		assert.Nil(t, err, "Failed spawning some procs: %v", err)
		ps = append(ps, p)
	}
	return ps
}

func spawnSleeperWithPid(t *testing.T, ts *test.Tstate, pid sp.Tpid) {
	spawnSleeperMcpu(t, ts, pid, 0, SLEEP_MSECS)
}

func spawnSleeper(t *testing.T, ts *test.Tstate) sp.Tpid {
	pid := sp.GenPid("sleeper")
	spawnSleeperWithPid(t, ts, pid)
	return pid
}

func spawnSleeperMcpu(t *testing.T, ts *test.Tstate, pid sp.Tpid, mcpu proc.Tmcpu, msecs int) {
	a := proc.NewProcPid(pid, "sleeper", []string{fmt.Sprintf("%dms", msecs), "name/"})
	a.SetMcpu(mcpu)
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
}

func spawnSpawner(t *testing.T, ts *test.Tstate, wait bool, childPid sp.Tpid, msecs, c int64) sp.Tpid {
	p := proc.NewProc("spawner", []string{strconv.FormatBool(wait), childPid.String(), "sleeper", fmt.Sprintf("%dms", msecs), "name/"})
	e0 := crash.Tevent{crash.SPAWNER_CRASH, 0, c, 0.33, 0}
	e1 := crash.Tevent{crash.SPAWNER_PARTITION, 0, c, 0.66, 0}
	s, err := crash.MakeTevents([]crash.Tevent{e0, e1})
	assert.Nil(t, err)
	p.AppendEnv(proc.SIGMAFAIL, s)
	err = ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	return p.GetPid()
}

func checkSleeperResult(t *testing.T, ts *test.Tstate, pid sp.Tpid) bool {
	res := true
	b, err := ts.GetFile("name/" + pid.String() + "_out")
	res = assert.Nil(t, err, "GetFile err: %v", err) && res
	res = assert.Equal(t, "hello", string(b), "Output") && res

	return res
}

func checkSleeperResultFalse(t *testing.T, ts *test.Tstate, pid sp.Tpid) {
	b, err := ts.GetFile("name/" + pid.String() + "_out")
	assert.NotNil(t, err, "GetFile")
	assert.NotEqual(t, string(b), "hello", "Output")
}

func cleanSleeperResult(t *testing.T, ts *test.Tstate, pid sp.Tpid) {
	ts.Remove("name/" + pid.String() + "_out")
}

func spawnWaitSleeper(ts *test.Tstate, kernels []string) *proc.Proc {
	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	if kernels != nil {
		a.SetKernels(kernels)
	}
	err := ts.Spawn(a)
	assert.Nil(ts.T, err, "Spawn")

	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(ts.T, err, "WaitExit error")
	assert.True(ts.T, status != nil && status.IsStatusOK(), "Exit status wrong")

	cleanSleeperResult(ts.T, ts, a.GetPid())
	return a
}

func TestCompile(t *testing.T) {
}

func TestErrStringCrashed(t *testing.T) {
	msg := `"{Err: "Non-sigma error" Obj: "" (exit status 3)}`
	err := serr.NewErrString(msg)
	assert.True(t, err.ErrCode == serr.TErrError)
	assert.Equal(t, err.Err.Error(), proc.CRASHSTATUS)
}

func TestWaitExitSimpleSingleBE(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	spawnWaitSleeper(ts, nil)
	ts.Shutdown()
}

func TestWaitExitSimpleSingleLC(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	a.SetMcpu(1000)
	db.DPrintf(db.TEST, "Pre spawn")
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong: %v", status)

	cleanSleeperResult(t, ts, a.GetPid())

	ts.Shutdown()
}

func TestWaitExitOne(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	start := time.Now()

	pid := spawnSleeper(t, ts)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong")

	// cleaned up (may take a bit)
	time.Sleep(500 * time.Millisecond)
	_, err = ts.Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat %v", filepath.Join(sp.PIDS, pid.String()))

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, pid)

	cleanSleeperResult(t, ts, pid)

	ts.Shutdown()
}

func TestWaitExitN(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	nProcs := 100
	var done sync.WaitGroup
	done.Add(nProcs)

	for i := 0; i < nProcs; i++ {
		go func() {
			pid := spawnSleeper(t, ts)
			status, err := ts.WaitExit(pid)
			assert.Nil(t, err, "WaitExit error")
			assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong %v", status)
			db.DPrintf(db.TEST, "Exited %v", pid)

			// cleaned up (may take a bit)
			time.Sleep(500 * time.Millisecond)
			_, err = ts.Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
			assert.NotNil(t, err, "Stat %v", filepath.Join(sp.PIDS, pid.String()))

			checkSleeperResult(t, ts, pid)
			cleanSleeperResult(t, ts, pid)

			done.Done()
		}()
	}
	done.Wait()
	ts.Shutdown()
}

func TestWaitExitParentRetStat(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	start := time.Now()

	pid := spawnSleeper(t, ts)
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong")

	// cleaned up
	for {
		_, err = ts.Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
		if err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
		db.DPrintf(db.TEST, "PID dir not deleted yet.")
	}
	assert.NotNil(t, err, "Stat %v", filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, pid)
	cleanSleeperResult(t, ts, pid)

	ts.Shutdown()
}

func TestWaitExitParentAbandons(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	start := time.Now()

	cPid := sp.GenPid("sleeper")
	pid := spawnSpawner(t, ts, false, cPid, SLEEP_MSECS, 0)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(pid)
	assert.True(t, status != nil && status.IsStatusOK(), "WaitExit status error")
	assert.Nil(t, err, "WaitExit error")
	// Wait for the child to run & finish
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

	// cleaned up
	_, err = ts.Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	ts.Shutdown()
}

func TestWaitExitParentCrash(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	start := time.Now()

	cPid := sp.GenPid("spawner")
	pid := spawnSpawner(t, ts, true, cPid, SLEEP_MSECS, CRASH_MSECS)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(pid)
	assert.True(t, status.IsStatusErr(), "WaitExit status not error: %v", status)
	assert.Nil(t, err, "WaitExit error")
	// Wait for the child to run & finish
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

	// cleaned up
	_, err = ts.Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	ts.Shutdown()
}

func TestWaitStart(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pid := spawnSleeper(t, ts)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")

	// Make sure the proc hasn't finished yet...
	checkSleeperResultFalse(t, ts, pid)

	ts.WaitExit(pid)

	// Make sure the proc finished...
	checkSleeperResult(t, ts, pid)

	cleanSleeperResult(t, ts, pid)

	ts.Shutdown()
}

// Should exit immediately
func TestWaitNonexistentProc(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ch := make(chan bool)

	pid := sp.GenPid("nonexistent")
	go func() {
		ts.WaitExit(pid)
		ch <- true
	}()

	done := <-ch
	assert.True(t, done, "Nonexistent proc")

	close(ch)

	ts.Shutdown()
}

func TestCrashProcOne(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	a := proc.NewProc("crash", []string{})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	err = ts.WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")

	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusErr(), "Status not err")
	sr := serr.NewErrString(status.Msg())
	assert.Equal(t, sr.Err.Error(), proc.CRASHSTATUS, "WaitExit")

	ts.Shutdown()
}

func TestEarlyExit1(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pid1 := sp.GenPid("parentexit")
	a := proc.NewProc("parentexit", []string{fmt.Sprintf("%dms", SLEEP_MSECS), pid1.String()})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	// Wait for parent to finish
	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusOK(), "WaitExit")

	// Child should not have terminated yet.
	checkSleeperResultFalse(t, ts, pid1)

	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

	// Child should have exited
	b, err := ts.GetFile("name/" + pid1.String() + "_out")
	assert.Nil(t, err, "GetFile")
	assert.Equal(t, "hello", string(b), "Output")

	// .. and cleaned up
	_, err = ts.Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid1.String()))
	assert.NotNil(t, err, "Stat")

	cleanSleeperResult(t, ts, pid1)

	ts.Shutdown()
}

func TestEarlyExitN(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	nProcs := 50 // 500
	const MAX_RETRY = 10
	var done sync.WaitGroup
	done.Add(nProcs)

	for i := 0; i < nProcs; i++ {
		go func(i int) {
			pid1 := sp.GenPid("parentexit")
			a := proc.NewProc("parentexit", []string{fmt.Sprintf("%dms", 0), pid1.String()})
			err := ts.Spawn(a)
			assert.Nil(t, err, "Spawn")

			// Wait for parent to finish
			status, err := ts.WaitExit(a.GetPid())
			assert.Nil(t, err, "WaitExit err: %v", err)
			assert.True(t, status != nil && status.IsStatusOK(), "WaitExit: %v", status)

			var gotfile bool
			var contentsCorrect bool
			var b []byte
			var err2 error
			for i := 0; i < MAX_RETRY && (!gotfile || !contentsCorrect); i++ {
				b, err2 = ts.GetFile("name/" + pid1.String() + "_out")
				gotfile = gotfile || err2 == nil
				contentsCorrect = contentsCorrect || string(b) == "hello"
				time.Sleep(time.Second)
			}

			// Child should have exited
			assert.True(t, gotfile, "GetFile failed: %v", err2)
			assert.True(t, contentsCorrect, "Incorrect file contents: %v", string(b))

			// .. and cleaned up
			_, err = ts.Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid1.String()))
			assert.NotNil(t, err, "Stat")

			cleanSleeperResult(t, ts, pid1)

			done.Done()
		}(i)
	}
	done.Wait()

	ts.Shutdown()
}

// Spawn a bunch of procs concurrently, then wait for all of them & check
// their result
func TestConcurrentProcs(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	nProcs := 8
	pids := map[sp.Tpid]int{}

	var barrier sync.WaitGroup
	barrier.Add(nProcs)
	var started sync.WaitGroup
	started.Add(nProcs)
	var done sync.WaitGroup
	done.Add(nProcs)

	for i := 0; i < nProcs; i++ {
		pid := sp.GenPid("sleeper")
		_, alreadySpawned := pids[pid]
		for alreadySpawned {
			pid = sp.GenPid("sleeper")
			_, alreadySpawned = pids[pid]
		}
		pids[pid] = i
		go func(pid sp.Tpid, started *sync.WaitGroup, i int) {
			barrier.Done()
			barrier.Wait()
			spawnSleeperWithPid(t, ts, pid)
			started.Done()
		}(pid, &started, i)
	}

	started.Wait()

	for pid, i := range pids {
		_ = i
		go func(pid sp.Tpid, done *sync.WaitGroup, i int) {
			defer done.Done()
			ts.WaitExit(pid)
			checkSleeperResult(t, ts, pid)
			cleanSleeperResult(t, ts, pid)
			time.Sleep(100 * time.Millisecond)
			_, err := ts.Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
			assert.NotNil(t, err, "Stat %v", filepath.Join(sp.PIDS, pid.String()))
		}(pid, &done, i)
	}

	done.Wait()

	ts.Shutdown()
}

func evict(ts *test.Tstate, pid sp.Tpid) {
	err := ts.WaitStart(pid)
	assert.Nil(ts.T, err, "Wait start err %v", err)
	time.Sleep(SLEEP_MSECS * time.Millisecond)
	err = ts.Evict(pid)
	assert.Nil(ts.T, err, "evict")
}

func TestEvict(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pid := spawnSpinner(t, ts)

	go evict(ts, pid)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusEvicted(), "WaitExit status")

	ts.Shutdown()
}

func TestEvictN(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	N := int(linuxsched.GetNCores())

	pids := []sp.Tpid{}
	for i := 0; i < N; i++ {
		pid := spawnSpinner(t, ts)
		pids = append(pids, pid)
		go evict(ts, pid)
	}

	for i := 0; i < N; i++ {
		status, err := ts.WaitExit(pids[i])
		assert.Nil(t, err, "WaitExit")
		assert.True(t, status != nil && status.IsStatusEvicted(), "WaitExit status")
	}

	ts.Shutdown()
}

func TestReserveCores(t *testing.T) {
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "SpawnBurst test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	start := time.Now()
	pid := sp.Tpid("sleeper-aaaaaaa")
	majorityCpu := 1000 * (linuxsched.GetNCores()/2 + 1)
	spawnSleeperMcpu(t, ts, pid, proc.Tmcpu(majorityCpu), SLEEP_MSECS)

	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")

	// Make sure pid1 is alphabetically sorted after pid, to ensure that this
	// proc is only picked up *after* the other one.
	pid1 := sp.Tpid("sleeper-bbbbbb")
	spawnSleeperMcpu(t, ts, pid1, proc.Tmcpu(majorityCpu), SLEEP_MSECS)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusOK(), "WaitExit status")

	// Make sure the second proc didn't finish
	checkSleeperResult(t, ts, pid)
	checkSleeperResultFalse(t, ts, pid1)

	cleanSleeperResult(t, ts, pid)

	status, err = ts.WaitExit(pid1)
	assert.Nil(t, err, "WaitExit 2")
	assert.True(t, status != nil && status.IsStatusOK(), "WaitExit status 2: %v", status)
	end := time.Now()

	assert.True(t, end.Sub(start) > (SLEEP_MSECS*2)*time.Millisecond, "Parallelized")

	checkSleeperResult(t, ts, pid1)

	cleanSleeperResult(t, ts, pid1)

	ts.Shutdown()
}

func TestWaitExitSimpleMultiKernel1(t *testing.T) {
	waitExitSimpleMultiKernel(t, 1)
}

func TestWaitExitSimpleMultiKernel3(t *testing.T) {
	waitExitSimpleMultiKernel(t, 3)
}

func waitExitSimpleMultiKernel(t *testing.T, n int) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	err = ts.BootNode(n)
	assert.Nil(t, err, "Boot node: %v", err)
	db.DPrintf(db.TEST, "Done boot node %d", n)

	sts, err := ts.GetDir(sp.MSCHED)
	kernels := sp.Names(sts)
	db.DPrintf(db.TEST, "Kernels %v", kernels)

	p := spawnWaitSleeper(ts, []string{kernels[0]})
	assert.Equal(t, kernels[0], p.GetKernelID())

	for i := 1; i < n+1; i++ {
		p := spawnWaitSleeper(ts, []string{kernels[i]})
		assert.Equal(t, kernels[i], p.GetKernelID())
	}

	ts.Shutdown()
}

func TestSpawnBurst(t *testing.T) {
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "SpawnBurst test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	// Number of spinners to burst-spawn
	N := (linuxsched.GetNCores()) * N_NODES

	// Start a couple new procds.
	for i := 0; i < N_NODES; i++ {
		err := ts.BootNode(1)
		assert.Nil(t, err, "BootNode %v", err)
	}

	db.DPrintf(db.TEST, "Start burst spawn %v", N)

	ps := burstSpawnSpinner(t, ts, 4)

	for _, p := range ps {
		err := ts.WaitStart(p.GetPid())
		assert.Nil(t, err, "WaitStart: %v", err)
	}

	db.DPrintf(db.TEST, "Evict burst spawn")

	for _, p := range ps {
		err := ts.Evict(p.GetPid())
		assert.Nil(t, err, "Evict: %v", err)
	}

	db.DPrintf(db.TEST, "Evict wait/exit spawn")

	for _, p := range ps {
		status, err := ts.WaitExit(p.GetPid())
		assert.Nil(t, err, "WaitExit: %v", err)
		assert.True(t, status != nil && status.IsStatusEvicted(), "%v: Wrong status: %v", p.GetPid(), status)
	}

	ts.Shutdown()
}

func TestSpawnManyProcsParallel(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	const N_CONCUR = 5  // 13
	const N_SPAWNS = 50 // 500

	err := ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	done := make(chan int)

	for i := 0; i < N_CONCUR; i++ {
		go func(i int) {
			for j := 0; j < N_SPAWNS; j++ {
				pid := sp.GenPid("sleeper")
				db.DPrintf(db.TEST, "Prep spawn %v", pid)
				a := proc.NewProcPid(pid, "sleeper", []string{"0ms", "name/"})
				err := ts.Spawn(a)
				assert.Nil(t, err, "Spawn err %v", err)
				db.DPrintf(db.TEST, "Done spawn %v", pid)

				db.DPrintf(db.TEST, "Prep WaitStart %v", pid)
				err = ts.WaitStart(a.GetPid())
				db.DPrintf(db.TEST, "Done WaitStart %v", pid)
				assert.Nil(t, err, "WaitStart error")

				db.DPrintf(db.TEST, "Prep WaitExit %v", pid)
				status, err := ts.WaitExit(a.GetPid())
				db.DPrintf(db.TEST, "Done WaitExit %v", pid)
				assert.Nil(t, err, "WaitExit")
				assert.True(t, status != nil && status.IsStatusOK(), "Status not OK: %v", status)
				cleanSleeperResult(t, ts, pid)
			}
			done <- i
		}(i)
	}
	for i := 0; i < N_CONCUR; i++ {
		x := <-done
		db.DPrintf(db.TEST, "Done %v", x)
	}

	ts.Shutdown()
}

func TestProcManyOK(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	a := proc.NewProc("proctest", []string{NTRIALS, "sleeper", "1us", ""})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "waitexit")
	assert.True(t, status.IsStatusOK(), status)
	ts.Shutdown()
}

func TestProcManyCrash(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	a := proc.NewProc("proctest", []string{NTRIALS, "crash"})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "waitexit")
	assert.True(t, status.IsStatusOK(), status)
	ts.Shutdown()
}

func TestProcManyPartition(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	a := proc.NewProc("proctest", []string{NTRIALS, "partition"})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "waitexit")
	if assert.NotNil(t, status, "nil status") {
		sr := serr.NewErrString(status.Msg())
		assert.Equal(t, sr.Err.Error(), proc.CRASHSTATUS, "WaitExit")
	}
	ts.Shutdown()
}

func TestSpawnCrashLCSched(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	db.DPrintf(db.TEST, "Spawn proc which will queue forever")

	// Spawn a proc which can't possibly be run by any msched.
	pid := spawnSpinnerMcpu(ts, proc.Tmcpu(1000*linuxsched.GetNCores()*2))

	db.DPrintf(db.TEST, "Kill a msched")

	err := ts.KillOne(sp.LCSCHEDREL)
	assert.Nil(t, err, "KillOne: %v", err)

	db.DPrintf(db.TEST, "Msched killed")

	err = ts.WaitStart(pid)
	assert.NotNil(t, err, "WaitStart: %v", err)

	db.DPrintf(db.TEST, "WaitStart done")

	_, err = ts.WaitExit(pid)
	assert.NotNil(t, err, "WaitExit: %v", err)

	db.DPrintf(db.TEST, "WaitExit done")

	ts.Shutdown()
}

// Make sure this test is still meaningful
func TestMaintainReplicationLevelCrashMSched(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	N_REPL := 3
	OUTDIR := "name/spinner-ephs"

	db.DPrintf(db.TEST, "Boot node 2")
	// Start a couple new nodes.
	err := ts.BootNode(1)
	assert.Nil(t, err, "BootNode %v", err)
	db.DPrintf(db.TEST, "Boot node 3")
	err = ts.BootNode(1)
	assert.Nil(t, err, "BootNode %v", err)
	db.DPrintf(db.TEST, "Done booting nodes")

	ts.RmDir(OUTDIR)
	err = ts.MkDir(OUTDIR, 0777)
	assert.Nil(t, err, "Mkdir")

	db.DPrintf(db.TEST, "Rm out dir done")

	// Start a bunch of replicated spinner procs.
	cfg := procgroupmgr.NewGroupConfig(N_REPL, "spinner", []string{}, 0, OUTDIR)
	sm := cfg.StartGrpMgr(ts.SigmaClnt)
	db.DPrintf(db.TEST, "GrpMgr started")

	// Wait for them to spawn.
	time.Sleep(5 * time.Second)

	// Make sure they spawned correctly.
	st, err := ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #1")
	db.DPrintf(db.TEST, "Get OutDir")

	err = ts.KillOne(sp.MSCHEDREL)
	assert.Nil(t, err, "kill msched")
	db.DPrintf(db.TEST, "Killed a msched")

	// Wait for them to respawn.
	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #2", sp.Names(st))
	db.DPrintf(db.TEST, "Got out dir again")

	err = ts.KillOne(sp.MSCHEDREL)
	assert.Nil(t, err, "kill msched")
	db.DPrintf(db.TEST, "Killed another msched")

	// Wait for them to respawn.
	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #3")
	db.DPrintf(db.TEST, "Got out dir 3")

	sm.StopGroup()
	db.DPrintf(db.TEST, "Stopped GroupMgr")

	// don't check for errors because between seeing the spinner file
	// exists and deleting it, the lease may have expired.
	ts.RmDir(OUTDIR)

	ts.Shutdown()
}
