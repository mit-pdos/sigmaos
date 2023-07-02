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
	"sigmaos/fslib"
	"sigmaos/groupmgr"
	"sigmaos/linuxsched"
	"sigmaos/perf"
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

func spawnSpinner(t *testing.T, ts *test.Tstate) proc.Tpid {
	return spawnSpinnerMcpu(ts, 0)
}

func spawnSpinnerMcpu(ts *test.Tstate, mcpu proc.Tmcpu) proc.Tpid {
	pid := proc.GenPid()
	a := proc.MakeProcPid(pid, "spinner", []string{"name/"})
	a.SetMcpu(mcpu)
	err := ts.Spawn(a)
	assert.Nil(ts.T, err, "Spawn")
	return pid
}

func burstSpawnSpinner(t *testing.T, ts *test.Tstate, N uint) []*proc.Proc {
	ps := make([]*proc.Proc, 0, N)
	for i := uint(0); i < N; i++ {
		p := proc.MakeProc("spinner", []string{"name/"})
		p.SetMcpu(1)
		ps = append(ps, p)
	}
	failed, errs := ts.SpawnBurst(ps, 1)
	assert.Equal(t, 0, len(failed), "Failed spawning some procs: %v", errs)
	return ps
}

func spawnSleeperWithPid(t *testing.T, ts *test.Tstate, pid proc.Tpid) {
	spawnSleeperMcpu(t, ts, pid, 0, SLEEP_MSECS)
}

func spawnSleeper(t *testing.T, ts *test.Tstate) proc.Tpid {
	pid := proc.GenPid()
	spawnSleeperWithPid(t, ts, pid)
	return pid
}

func spawnSleeperMcpu(t *testing.T, ts *test.Tstate, pid proc.Tpid, mcpu proc.Tmcpu, msecs int) {
	a := proc.MakeProcPid(pid, "sleeper", []string{fmt.Sprintf("%dms", msecs), "name/"})
	a.SetMcpu(mcpu)
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
}

func spawnSpawner(t *testing.T, ts *test.Tstate, wait bool, childPid proc.Tpid, msecs, crash int) proc.Tpid {
	p := proc.MakeProc("spawner", []string{strconv.FormatBool(wait), childPid.String(), "sleeper", fmt.Sprintf("%dms", msecs), "name/"})
	p.AppendEnv(proc.SIGMACRASH, strconv.Itoa(crash))
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	return p.GetPid()
}

func checkSleeperResult(t *testing.T, ts *test.Tstate, pid proc.Tpid) bool {
	res := true
	b, err := ts.GetFile("name/" + pid.String() + "_out")
	res = assert.Nil(t, err, "GetFile") && res
	res = assert.Equal(t, string(b), "hello", "Output") && res

	return res
}

func checkSleeperResultFalse(t *testing.T, ts *test.Tstate, pid proc.Tpid) {
	b, err := ts.GetFile("name/" + pid.String() + "_out")
	assert.NotNil(t, err, "GetFile")
	assert.NotEqual(t, string(b), "hello", "Output")
}

func TestWaitExitSimpleSingle(t *testing.T) {
	ts := test.MakeTstateAll(t)
	a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong: %v", status)

	ts.Shutdown()
}

func TestWaitExitSimpleMultiKernel(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.BootNode(1)
	assert.Nil(t, err, "Boot node: %v", err)

	a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts.Spawn(a)
	db.DPrintf(db.TEST, "Post spawn")
	assert.Nil(t, err, "Spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong")

	ts.Shutdown()
}

func TestWaitExitOne(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong")

	// cleaned up (may take a bit)
	time.Sleep(500 * time.Millisecond)
	_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat %v", path.Join(sp.PIDS, pid.String()))

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, pid)

	ts.Shutdown()
}

func TestWaitExitN(t *testing.T) {
	ts := test.MakeTstateAll(t)
	nProcs := 100
	var done sync.WaitGroup
	done.Add(nProcs)

	for i := 0; i < nProcs; i++ {
		go func() {
			pid := spawnSleeper(t, ts)
			status, err := ts.WaitExit(pid)
			assert.Nil(t, err, "WaitExit error")
			assert.True(t, status.IsStatusOK(), "Exit status wrong %v", status)
			db.DPrintf(db.TEST, "Exited %v", pid)

			// cleaned up (may take a bit)
			time.Sleep(500 * time.Millisecond)
			_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
			assert.NotNil(t, err, "Stat %v", path.Join(sp.PIDS, pid.String()))

			checkSleeperResult(t, ts, pid)

			done.Done()
		}()
	}
	done.Wait()
	ts.Shutdown()
}

func TestWaitExitParentRetStat(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong")

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

	ts.Shutdown()
}

func TestWaitExitParentAbandons(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()

	cPid := proc.GenPid()
	pid := spawnSpawner(t, ts, false, cPid, SLEEP_MSECS, 0)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(pid)
	assert.True(t, status.IsStatusOK(), "WaitExit status error")
	assert.Nil(t, err, "WaitExit error")
	// Wait for the child to run & finish
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

	// cleaned up
	_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, cPid)

	ts.Shutdown()
}

func TestWaitExitParentCrash(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()

	cPid := proc.GenPid()
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

	checkSleeperResult(t, ts, cPid)

	ts.Shutdown()
}

func TestWaitStart(t *testing.T) {
	ts := test.MakeTstateAll(t)

	pid := spawnSleeper(t, ts)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")

	// Check if proc exists
	sts, err := ts.GetDir(path.Join(sp.SCHEDD, schedd(ts), sp.RUNNING))
	assert.Nil(t, err, "Readdir")
	assert.True(t, fslib.Present(sts, []string{pid.String()}), "pid")

	// Make sure the proc hasn't finished yet...
	checkSleeperResultFalse(t, ts, pid)

	ts.WaitExit(pid)

	// Make sure the proc finished...
	checkSleeperResult(t, ts, pid)

	ts.Shutdown()
}

// Should exit immediately
func TestWaitNonexistentProc(t *testing.T) {
	ts := test.MakeTstateAll(t)

	ch := make(chan bool)

	pid := proc.GenPid()
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
	ts := test.MakeTstateAll(t)

	const N_CONCUR = 13
	const N_SPAWNS = 500

	err := ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	done := make(chan int)

	for i := 0; i < N_CONCUR; i++ {
		go func(i int) {
			for j := 0; j < N_SPAWNS; j++ {
				pid := proc.GenPid()
				db.DPrintf(db.TEST, "Prep spawn %v", pid)
				a := proc.MakeProcPid(pid, "sleeper", []string{"0ms", "name/"})
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
				assert.True(t, status.IsStatusOK(), "Status not OK")
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
	ts := test.MakeTstateAll(t)

	a := proc.MakeProc("crash", []string{})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	err = ts.WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")

	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusErr(), "Status not err")
	assert.Equal(t, "exit status 2", status.Msg(), "WaitExit")

	ts.Shutdown()
}

func TestEarlyExit1(t *testing.T) {
	ts := test.MakeTstateAll(t)

	pid1 := proc.GenPid()
	a := proc.MakeProc("parentexit", []string{fmt.Sprintf("%dms", SLEEP_MSECS), pid1.String()})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	// Wait for parent to finish
	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusOK(), "WaitExit")

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

	ts.Shutdown()
}

func TestEarlyExitN(t *testing.T) {
	ts := test.MakeTstateAll(t)
	nProcs := 500
	var done sync.WaitGroup
	done.Add(nProcs)

	for i := 0; i < nProcs; i++ {
		go func(i int) {
			pid1 := proc.GenPid()
			a := proc.MakeProc("parentexit", []string{fmt.Sprintf("%dms", 0), pid1.String()})
			err := ts.Spawn(a)
			assert.Nil(t, err, "Spawn")

			// Wait for parent to finish
			status, err := ts.WaitExit(a.GetPid())
			assert.Nil(t, err, "WaitExit err: %v", err)
			assert.True(t, status.IsStatusOK(), "WaitExit: %v", status)

			time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

			// Child should have exited
			b, err := ts.GetFile("name/" + pid1.String() + "_out")
			assert.Nil(t, err, "GetFile")
			assert.Equal(t, string(b), "hello", "Output")

			// .. and cleaned up
			_, err = ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid1.String()))
			assert.NotNil(t, err, "Stat")
			done.Done()
		}(i)
	}
	done.Wait()

	ts.Shutdown()
}

// Spawn a bunch of procs concurrently, then wait for all of them & check
// their result
func TestConcurrentProcs(t *testing.T) {
	ts := test.MakeTstateAll(t)

	nProcs := 8
	pids := map[proc.Tpid]int{}

	var barrier sync.WaitGroup
	barrier.Add(nProcs)
	var started sync.WaitGroup
	started.Add(nProcs)
	var done sync.WaitGroup
	done.Add(nProcs)

	for i := 0; i < nProcs; i++ {
		pid := proc.GenPid()
		_, alreadySpawned := pids[pid]
		for alreadySpawned {
			pid = proc.GenPid()
			_, alreadySpawned = pids[pid]
		}
		pids[pid] = i
		go func(pid proc.Tpid, started *sync.WaitGroup, i int) {
			barrier.Done()
			barrier.Wait()
			spawnSleeperWithPid(t, ts, pid)
			started.Done()
		}(pid, &started, i)
	}

	started.Wait()

	for pid, i := range pids {
		_ = i
		go func(pid proc.Tpid, done *sync.WaitGroup, i int) {
			defer done.Done()
			ts.WaitExit(pid)
			checkSleeperResult(t, ts, pid)
			time.Sleep(100 * time.Millisecond)
			_, err := ts.Stat(path.Join(sp.SCHEDD, "~local", sp.PIDS, pid.String()))
			assert.NotNil(t, err, "Stat %v", path.Join(sp.PIDS, pid.String()))
		}(pid, &done, i)
	}

	done.Wait()

	ts.Shutdown()
}

func evict(ts *test.Tstate, pid proc.Tpid) {
	err := ts.WaitStart(pid)
	assert.Nil(ts.T, err, "Wait start err %v", err)
	time.Sleep(SLEEP_MSECS * time.Millisecond)
	err = ts.Evict(pid)
	assert.Nil(ts.T, err, "evict")
}

func TestEvict(t *testing.T) {
	ts := test.MakeTstateAll(t)

	pid := spawnSpinner(t, ts)

	go evict(ts, pid)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusEvicted(), "WaitExit status")

	ts.Shutdown()
}

func TestReserveCores(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()
	pid := proc.Tpid("sleeper-aaaaaaa")
	spawnSleeperMcpu(t, ts, pid, proc.Tmcpu(1000*linuxsched.NCores), SLEEP_MSECS)

	// Make sure pid1 is alphabetically sorted after pid, to ensure that this
	// proc is only picked up *after* the other one.
	pid1 := proc.Tpid("sleeper-bbbbbb")
	spawnSleeperMcpu(t, ts, pid1, 1000, SLEEP_MSECS)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusOK(), "WaitExit status")

	// Make sure the second proc didn't finish
	checkSleeperResult(t, ts, pid)
	checkSleeperResultFalse(t, ts, pid1)

	status, err = ts.WaitExit(pid1)
	assert.Nil(t, err, "WaitExit 2")
	assert.True(t, status.IsStatusOK(), "WaitExit status 2")
	end := time.Now()

	assert.True(t, end.Sub(start) > (SLEEP_MSECS*2)*time.Millisecond, "Parallelized")

	ts.Shutdown()
}

func TestWorkStealing(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.BootNode(1)
	assert.Nil(t, err, "Boot node %v", err)

	pid := spawnSpinnerMcpu(ts, proc.Tmcpu(1000*linuxsched.NCores))
	pid1 := spawnSpinnerMcpu(ts, proc.Tmcpu(1000*linuxsched.NCores))

	err = ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart")

	err = ts.WaitStart(pid1)
	assert.Nil(t, err, "WaitStart")

	err = ts.Evict(pid)
	assert.Nil(t, err, "Evict")

	err = ts.Evict(pid1)
	assert.Nil(t, err, "Evict")

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusEvicted(), "WaitExit status")

	status, err = ts.WaitExit(pid1)
	assert.Nil(t, err, "WaitExit 2")
	assert.True(t, status.IsStatusEvicted(), "WaitExit status 2")

	// Check that work-stealing symlinks were cleaned up.
	sts, _, err := ts.ReadDir(sp.WS_RUNQ_LC)
	assert.Nil(t, err, "Readdir %v", err)
	assert.Equal(t, 0, len(sts), "Wrong length ws dir[%v]: %v", sp.WS_RUNQ_LC, sts)

	sts, _, err = ts.ReadDir(sp.WS_RUNQ_BE)
	assert.Nil(t, err, "Readdir %v", err)
	assert.Equal(t, 0, len(sts), "Wrong length ws dir[%v]: %v", sp.WS_RUNQ_BE, sts)

	ts.Shutdown()
}

func TestEvictN(t *testing.T) {
	ts := test.MakeTstateAll(t)

	N := int(linuxsched.NCores)

	pids := []proc.Tpid{}
	for i := 0; i < N; i++ {
		pid := spawnSpinner(t, ts)
		pids = append(pids, pid)
		go evict(ts, pid)
	}

	for i := 0; i < N; i++ {
		status, err := ts.WaitExit(pids[i])
		assert.Nil(t, err, "WaitExit")
		assert.True(t, status.IsStatusEvicted(), "WaitExit status")
	}

	ts.Shutdown()
}

func getNChildren(ts *test.Tstate) int {
	c, err := ts.GetChildren()
	assert.Nil(ts.T, err, "getnchildren")
	return len(c)
}

func TestBurstSpawn(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Number of spinners to burst-spawn
	N := (linuxsched.NCores) * 3

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
		assert.True(t, status.IsStatusEvicted(), "Wrong status: %v", status)
	}

	ts.Shutdown()
}

func TestSpawnCrashSchedd(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Spawn a proc which can't possibly be run by any procd.
	pid := spawnSpinnerMcpu(ts, proc.Tmcpu(1000*linuxsched.NCores*2))

	err := ts.KillOne(sp.SCHEDDREL)
	assert.Nil(t, err, "KillOne: %v", err)

	err = ts.WaitStart(pid)
	assert.NotNil(t, err, "WaitStart: %v", err)

	_, err = ts.WaitExit(pid)
	assert.NotNil(t, err, "WaitExit: %v", err)

	ts.Shutdown()
}

func TestMaintainReplicationLevelCrashSchedd(t *testing.T) {
	ts := test.MakeTstateAll(t)

	N_REPL := 3
	OUTDIR := "name/spinner-ephs"

	// Start a couple new nodes.
	err := ts.BootNode(1)
	assert.Nil(t, err, "BootNode %v", err)
	err = ts.BootNode(1)
	assert.Nil(t, err, "BootNode %v", err)

	// Count number of children.
	nChildren := getNChildren(ts)

	err = ts.MkDir(OUTDIR, 0777)
	assert.Nil(t, err, "Mkdir")

	// Start a bunch of replicated spinner procs.
	sm := groupmgr.Start(ts.SigmaClnt, N_REPL, "spinner", []string{}, OUTDIR, 0, N_REPL, 0, 0, 0)
	nChildren += N_REPL

	// Wait for them to spawn.
	time.Sleep(1 * time.Second)

	// Make sure they spawned correctly.
	st, err := ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #1")
	assert.Equal(t, nChildren, getNChildren(ts), "wrong num children")

	err = ts.KillOne(sp.SCHEDDREL)
	assert.Nil(t, err, "kill schedd")

	// Wait for them to respawn.
	time.Sleep(5 * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #2")

	err = ts.KillOne(sp.SCHEDDREL)
	assert.Nil(t, err, "kill schedd")

	// Wait for them to respawn.
	time.Sleep(5 * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #3")

	sm.Stop()

	ts.Shutdown()
}

// Test to see if any core has a spinner running on it (high utilization).
func anyCoresOccupied(coresMaps []map[string]bool) bool {
	N_SAMPLES := 5
	// Calculate the average utilization over a 250ms period for each core to be
	// revoked.
	coreOccupied := false
	for c, m := range coresMaps {
		idle0, total0 := perf.GetCPUSample(m)
		idleDelta := uint64(0)
		totalDelta := uint64(0)
		// Collect some CPU util samples for this core.
		for i := 0; i < N_SAMPLES; i++ {
			time.Sleep(25 * time.Millisecond)
			idle1, total1 := perf.GetCPUSample(m)
			idleDelta += idle1 - idle0
			totalDelta += total1 - total0
			idle0 = idle1
			total0 = total1
		}
		avgCoreUtil := 100.0 * ((float64(totalDelta) - float64(idleDelta)) / float64(totalDelta))
		db.DPrintf(db.TEST, "Core %v utilization: %v", c, avgCoreUtil)
		if avgCoreUtil > 50.0 {
			coreOccupied = true
		}
	}
	return coreOccupied
}
