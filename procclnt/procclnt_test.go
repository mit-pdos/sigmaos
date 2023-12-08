package procclnt_test

import (
	"fmt"
	"path"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/groupmgr"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SLEEP_MSECS = 2000
	CRASH_MSECS = 5
)

const program = "procclnt_test"

func schedd(ts *test.Tstate) string {
	st, err := ts.GetDir(sp.SCHEDD)
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

func burstSpawnSpinner(t *testing.T, ts *test.Tstate, N uint) []*proc.Proc {
	ps := make([]*proc.Proc, 0, N)
	for i := uint(0); i < N; i++ {
		p := proc.NewProc("spinner", []string{"name/"})
		p.SetMcpu(1000)
		ps = append(ps, p)
	}
	failed, errs := ts.SpawnBurst(ps, 1)
	assert.Equal(t, 0, len(failed), "Failed spawning some procs: %v", errs)
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

func spawnSpawner(t *testing.T, ts *test.Tstate, wait bool, childPid sp.Tpid, msecs, crash int) sp.Tpid {
	p := proc.NewProc("spawner", []string{strconv.FormatBool(wait), childPid.String(), "sleeper", fmt.Sprintf("%dms", msecs), "name/"})
	p.SetCrash(int64(crash))
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	return p.GetPid()
}

func checkSleeperResult(t *testing.T, ts *test.Tstate, pid sp.Tpid) bool {
	res := true
	b, err := ts.GetFile("name/" + pid.String() + "_out")
	res = assert.Nil(t, err, "GetFile err: %v", err) && res
	res = assert.Equal(t, string(b), "hello", "Output") && res

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

func TestWaitExitSimpleSingleBE(t *testing.T) {
	ts := test.NewTstateAll(t)
	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
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
	// test.Dump(t)
}

func TestWaitExitSimpleSingleLC(t *testing.T) {
	ts := test.NewTstateAll(t)
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

func TestWaitExitSimpleMultiKernel(t *testing.T) {
	ts := test.NewTstateAll(t)

	err := ts.BootNode(1)
	assert.Nil(t, err, "Boot node: %v", err)

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts.Spawn(a)
	db.DPrintf(db.TEST, "Post spawn")
	assert.Nil(t, err, "Spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong")

	cleanSleeperResult(t, ts, a.GetPid())

	ts.Shutdown()
}

func TestWaitExitOne(t *testing.T) {
	ts := test.NewTstateAll(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong")

	// cleaned up (may take a bit)
	time.Sleep(500 * time.Millisecond)
	_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat %v", path.Join(sp.PIDS, pid.String()))

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, pid)

	cleanSleeperResult(t, ts, pid)

	ts.Shutdown()
}

func TestWaitExitN(t *testing.T) {
	ts := test.NewTstateAll(t)
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
			_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
			assert.NotNil(t, err, "Stat %v", path.Join(sp.PIDS, pid.String()))

			checkSleeperResult(t, ts, pid)
			cleanSleeperResult(t, ts, pid)

			done.Done()
		}()
	}
	done.Wait()
	ts.Shutdown()
}

func TestWaitExitParentRetStat(t *testing.T) {
	ts := test.NewTstateAll(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong")

	// cleaned up
	for {
		_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
		if err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
		db.DPrintf(db.TEST, "PID dir not deleted yet.")
	}
	assert.NotNil(t, err, "Stat %v", path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, pid)
	cleanSleeperResult(t, ts, pid)

	ts.Shutdown()
}

func TestWaitExitParentAbandons(t *testing.T) {
	ts := test.NewTstateAll(t)

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
	_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	ts.Shutdown()
}

func TestWaitExitParentCrash(t *testing.T) {
	ts := test.NewTstateAll(t)

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
	_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	ts.Shutdown()
}

func TestWaitStart(t *testing.T) {
	ts := test.NewTstateAll(t)

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
	ts := test.NewTstateAll(t)

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

func TestSpawnManyProcsParallel(t *testing.T) {
	ts := test.NewTstateAll(t)

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
				_, errs := ts.SpawnBurst([]*proc.Proc{a}, 1)
				assert.True(t, len(errs) == 0, "Spawn err %v", errs)
				db.DPrintf(db.TEST, "Done spawn %v", pid)

				db.DPrintf(db.TEST, "Prep WaitStart %v", pid)
				err := ts.WaitStart(a.GetPid())
				db.DPrintf(db.TEST, "Done WaitStart %v", pid)
				assert.Nil(t, err, "WaitStart error")

				db.DPrintf(db.TEST, "Prep WaitExit %v", pid)
				status, err := ts.WaitExit(a.GetPid())
				db.DPrintf(db.TEST, "Done WaitExit %v", pid)
				assert.Nil(t, err, "WaitExit")
				assert.True(t, status != nil && status.IsStatusOK(), "Status not OK")
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

func TestCrashProcOne(t *testing.T) {
	ts := test.NewTstateAll(t)

	a := proc.NewProc("crash", []string{})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	err = ts.WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")

	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusErr(), "Status not err")
	assert.Equal(t, "Non-sigma error  Non-sigma error  exit status 2", status.Msg(), "WaitExit")

	ts.Shutdown()
}

func TestEarlyExit1(t *testing.T) {
	ts := test.NewTstateAll(t)

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
	assert.Equal(t, string(b), "hello", "Output")

	// .. and cleaned up
	_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid1.String()))
	assert.NotNil(t, err, "Stat")

	cleanSleeperResult(t, ts, pid1)

	ts.Shutdown()
}

func TestEarlyExitN(t *testing.T) {
	ts := test.NewTstateAll(t)
	nProcs := 50 // 500
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

			time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

			// Child should have exited
			b, err := ts.GetFile("name/" + pid1.String() + "_out")
			assert.Nil(t, err, "GetFile")
			assert.Equal(t, string(b), "hello", "Output")

			// .. and cleaned up
			_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid1.String()))
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
	ts := test.NewTstateAll(t)

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
			_, err := ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
			assert.NotNil(t, err, "Stat %v", path.Join(sp.PIDS, pid.String()))
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
	ts := test.NewTstateAll(t)

	pid := spawnSpinner(t, ts)

	go evict(ts, pid)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusEvicted(), "WaitExit status")

	ts.Shutdown()
}

func TestEvictN(t *testing.T) {
	ts := test.NewTstateAll(t)

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

func TestBurstSpawn(t *testing.T) {
	ts := test.NewTstateAll(t)

	// Number of spinners to burst-spawn
	N := (linuxsched.GetNCores()) * 3

	// Start a couple new procds.
	err := ts.BootNode(1)
	assert.Nil(t, err, "BootNode %v", err)
	err = ts.BootNode(1)
	assert.Nil(t, err, "BootNode %v", err)

	db.DPrintf(db.TEST, "Start burst spawn")

	ps := burstSpawnSpinner(t, ts, N)

	for _, p := range ps {
		err := ts.WaitStart(p.GetPid())
		assert.Nil(t, err, "WaitStart: %v", err)
	}

	for _, p := range ps {
		err := ts.Evict(p.GetPid())
		assert.Nil(t, err, "Evict: %v", err)
	}

	for _, p := range ps {
		status, err := ts.WaitExit(p.GetPid())
		assert.Nil(t, err, "WaitExit: %v", err)
		assert.True(t, status != nil && status.IsStatusEvicted(), "Wrong status: %v", status)
	}

	ts.Shutdown()
}

func TestReserveCores(t *testing.T) {
	ts := test.NewTstateAll(t)

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

func TestSpawnCrashLCSched(t *testing.T) {
	ts := test.NewTstateAll(t)

	db.DPrintf(db.TEST, "Spawn proc which will queue forever")

	// Spawn a proc which can't possibly be run by any schedd.
	pid := spawnSpinnerMcpu(ts, proc.Tmcpu(1000*linuxsched.GetNCores()*2))

	db.DPrintf(db.TEST, "Kill a schedd")

	err := ts.KillOne(sp.LCSCHEDREL)
	assert.Nil(t, err, "KillOne: %v", err)

	db.DPrintf(db.TEST, "Schedd killed")

	err = ts.WaitStart(pid)
	assert.NotNil(t, err, "WaitStart: %v", err)

	db.DPrintf(db.TEST, "WaitStart done")

	_, err = ts.WaitExit(pid)
	assert.NotNil(t, err, "WaitExit: %v", err)

	db.DPrintf(db.TEST, "WaitExit done")

	ts.Shutdown()
}

// Make sure this test is still meaningful
func TestMaintainReplicationLevelCrashSchedd(t *testing.T) {
	ts := test.NewTstateAll(t)

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
	cfg := groupmgr.NewGroupConfig(N_REPL, "spinner", []string{}, 0, OUTDIR)
	sm := cfg.StartGrpMgr(ts.SigmaClnt, 0)
	db.DPrintf(db.TEST, "GrpMgr started")

	// Wait for them to spawn.
	time.Sleep(5 * time.Second)

	// Make sure they spawned correctly.
	st, err := ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #1")
	db.DPrintf(db.TEST, "Get OutDir")

	err = ts.KillOne(sp.SCHEDDREL)
	assert.Nil(t, err, "kill schedd")
	db.DPrintf(db.TEST, "Killed a schedd")

	// Wait for them to respawn.
	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #2")
	db.DPrintf(db.TEST, "Got out dir again")

	err = ts.KillOne(sp.SCHEDDREL)
	assert.Nil(t, err, "kill schedd")
	db.DPrintf(db.TEST, "Killed another schedd")

	// Wait for them to respawn.
	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #3")
	db.DPrintf(db.TEST, "Got out dir 3")

	sm.StopGroup()
	db.DPrintf(db.TEST, "Stopped GroupMgr")

	err = ts.RmDir(OUTDIR)
	assert.Nil(t, err, "RmDir: %v", err)
	db.DPrintf(db.TEST, "Get out dir 4")

	ts.Shutdown()
}
