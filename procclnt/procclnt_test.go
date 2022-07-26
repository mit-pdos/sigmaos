package procclnt_test

import (
	"fmt"
	"math"
	"path"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/groupmgr"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/resource"
	"ulambda/stats"
	"ulambda/test"
)

const (
	SLEEP_MSECS = 2000
)

const program = "procclnt_test"

func procd(ts *test.Tstate) string {
	st, err := ts.GetDir("name/procd")
	assert.Nil(ts.T, err, "Readdir")
	return st[0].Name
}

func spawnSpinner(t *testing.T, ts *test.Tstate) proc.Tpid {
	return spawnSpinnerNcore(t, ts, proc.C_DEF)
}

func spawnSpinnerNcore(t *testing.T, ts *test.Tstate, ncore proc.Tcore) proc.Tpid {
	pid := proc.GenPid()
	a := proc.MakeProcPid(pid, "user/spinner", []string{"name/"})
	a.SetNcore(ncore)
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return pid
}

func burstSpawnSpinner(t *testing.T, ts *test.Tstate, N uint) []*proc.Proc {
	ps := make([]*proc.Proc, 0, N)
	for i := uint(0); i < N; i++ {
		p := proc.MakeProc("user/spinner", []string{"name/"})
		p.SetNcore(1)
		ps = append(ps, p)
	}
	failed, errs := ts.SpawnBurst(ps)
	assert.Equal(t, 0, len(failed), "Failed spawning some procs: %v", errs)
	return ps
}

func spawnSleeperWithPid(t *testing.T, ts *test.Tstate, pid proc.Tpid) {
	spawnSleeperNcore(t, ts, pid, 0, SLEEP_MSECS)
}

func spawnSleeper(t *testing.T, ts *test.Tstate) proc.Tpid {
	pid := proc.GenPid()
	spawnSleeperWithPid(t, ts, pid)
	return pid
}

func spawnSleeperNcore(t *testing.T, ts *test.Tstate, pid proc.Tpid, ncore proc.Tcore, msecs int) {
	a := proc.MakeProcPid(pid, "user/sleeper", []string{fmt.Sprintf("%dms", msecs), "name/"})
	a.SetNcore(ncore)
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
}

func spawnSpawner(t *testing.T, ts *test.Tstate, childPid proc.Tpid, msecs int) proc.Tpid {
	p := proc.MakeProc("user/spawner", []string{"false", childPid.String(), "user/sleeper", fmt.Sprintf("%dms", msecs), "name/"})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	return p.Pid
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

func TestWaitExitOne(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong")

	// cleaned up (may take a bit)
	time.Sleep(500 * time.Millisecond)
	_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat %v", path.Join(proc.PIDS, pid.String()))

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

			// cleaned up (may take a bit)
			time.Sleep(500 * time.Millisecond)
			_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid.String()))
			assert.NotNil(t, err, "Stat %v", path.Join(proc.PIDS, pid.String()))

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
	_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat %v", path.Join(np.PROCD, "~ip", proc.PIDS, pid.String()))

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, pid)

	ts.Shutdown()
}

func TestWaitExitParentAbandons(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()

	cPid := proc.GenPid()
	pid := spawnSpawner(t, ts, cPid, SLEEP_MSECS)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(pid)
	assert.True(t, status.IsStatusOK(), "WaitExit status error")
	assert.Nil(t, err, "WaitExit error")
	// Wait for the child to run & finish
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

	// cleaned up
	_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, cPid)

	ts.Shutdown()
}

func TestWaitStart(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")

	end := time.Now()

	assert.True(t, end.Sub(start) < SLEEP_MSECS*time.Millisecond, "WaitStart waited too long")

	// Check if proc exists
	sts, err := ts.GetDir(path.Join("name/procd", procd(ts), np.PROCD_RUNNING))
	assert.Nil(t, err, "Readdir")
	assert.True(t, fslib.Present(sts, []string{pid.String()}), "pid")

	// Make sure the proc hasn't finished yet...
	checkSleeperResultFalse(t, ts, pid)

	ts.WaitExit(pid)

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

func TestCrashProcOne(t *testing.T) {
	ts := test.MakeTstateAll(t)

	a := proc.MakeProc("user/crash", []string{})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	err = ts.WaitStart(a.Pid)
	assert.Nil(t, err, "WaitStart error")

	status, err := ts.WaitExit(a.Pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusErr(), "Status not err")
	assert.Equal(t, "exit status 2", status.Msg(), "WaitExit")

	ts.Shutdown()
}

func TestEarlyExit1(t *testing.T) {
	ts := test.MakeTstateAll(t)

	pid1 := proc.GenPid()
	a := proc.MakeProc("user/parentexit", []string{fmt.Sprintf("%dms", SLEEP_MSECS), pid1.String()})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	// Wait for parent to finish
	status, err := ts.WaitExit(a.Pid)
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
	_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid1.String()))
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
			a := proc.MakeProc("user/parentexit", []string{fmt.Sprintf("%dms", 0), pid1.String()})
			err := ts.Spawn(a)
			assert.Nil(t, err, "Spawn")

			// Wait for parent to finish
			status, err := ts.WaitExit(a.Pid)
			assert.Nil(t, err, "WaitExit err: %v", err)
			assert.True(t, status.IsStatusOK(), "WaitExit: %v", status)

			time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

			// Child should have exited
			b, err := ts.GetFile("name/" + pid1.String() + "_out")
			assert.Nil(t, err, "GetFile")
			assert.Equal(t, string(b), "hello", "Output")

			// .. and cleaned up
			_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid1.String()))
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
			_, err := ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid.String()))
			assert.NotNil(t, err, "Stat %v", path.Join(proc.PIDS, pid.String()))
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
	spawnSleeperNcore(t, ts, pid, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)

	// Make sure pid1 is alphabetically sorted after pid, to ensure that this
	// proc is only picked up *after* the other one.
	pid1 := proc.Tpid("sleeper-bbbbbb")
	spawnSleeperNcore(t, ts, pid1, 1, SLEEP_MSECS)

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

	ts.BootProcd()

	start := time.Now()
	pid := proc.GenPid()
	spawnSleeperNcore(t, ts, pid, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)

	pid1 := proc.GenPid()
	spawnSleeperNcore(t, ts, pid1, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusOK(), "WaitExit status")

	status, err = ts.WaitExit(pid1)
	assert.Nil(t, err, "WaitExit 2")
	assert.True(t, status.IsStatusOK(), "WaitExit status 2")
	end := time.Now()

	// Make sure both procs finished
	checkSleeperResult(t, ts, pid)
	checkSleeperResult(t, ts, pid1)

	assert.True(t, end.Sub(start) < (SLEEP_MSECS*2)*time.Millisecond, "Parallelized: took too long (%v msec)", end.Sub(start).Milliseconds())

	// Check that work-stealing symlinks were cleaned up.
	sts, _, err := ts.ReadDir(np.PROCD_WS)
	assert.Nil(t, err, "Readdir %v", err)
	assert.Equal(t, 0, len(sts), "Wrong length ws dir: %v", sts)

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
	N := linuxsched.NCores * 3

	// Start a couple new procds.
	err := ts.BootProcd()
	assert.Nil(t, err, "BootProcd 1")
	err = ts.BootProcd()
	assert.Nil(t, err, "BootProcd 2")

	ps := burstSpawnSpinner(t, ts, N)

	for _, p := range ps {
		err := ts.WaitStart(p.Pid)
		assert.Nil(t, err, "WaitStart: %v", err)
	}

	for _, p := range ps {
		err := ts.Evict(p.Pid)
		assert.Nil(t, err, "Evict: %v", err)
	}

	for _, p := range ps {
		status, err := ts.WaitExit(p.Pid)
		assert.Nil(t, err, "WaitExit: %v", err)
		assert.True(t, status.IsStatusEvicted(), "Wrong status: %v", status)
	}

	ts.Shutdown()
}

func TestMaintainReplicationLevelCrashProcd(t *testing.T) {
	ts := test.MakeTstateAll(t)

	N_REPL := 3
	OUTDIR := "name/spinner-ephs"

	// Start a couple new procds.
	err := ts.BootProcd()
	assert.Nil(t, err, "BootProcd 1")
	err = ts.BootProcd()
	assert.Nil(t, err, "BootProcd 2")

	// Count number of children.
	nChildren := getNChildren(ts)

	err = ts.MkDir(OUTDIR, 0777)
	assert.Nil(t, err, "Mkdir")

	// Start a bunch of replicated spinner procs.
	sm := groupmgr.Start(ts.FsLib, ts.ProcClnt, N_REPL, "user/spinner", []string{OUTDIR}, 0, N_REPL, 0, 0, 0)
	nChildren += N_REPL

	// Wait for them to spawn.
	time.Sleep(1 * time.Second)

	// Make sure they spawned correctly.
	st, err := ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #1")
	assert.Equal(t, nChildren, getNChildren(ts), "wrong num children")

	err = ts.KillOne(np.PROCD)
	assert.Nil(t, err, "kill procd")

	// Wait for them to respawn.
	time.Sleep(5 * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #2")

	err = ts.KillOne(np.PROCD)
	assert.Nil(t, err, "kill procd")

	// Wait for them to respawn.
	time.Sleep(1 * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.GetDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #3")

	sm.Stop()

	ts.Shutdown()
}

func TestProcdResize1(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Run a proc that claims all cores.
	pid := proc.GenPid()
	spawnSleeperNcore(t, ts, pid, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusOK(), "WaitExit status")
	checkSleeperResult(t, ts, pid)

	nCoresToRevoke := int(math.Ceil(float64(linuxsched.NCores)/2 + 1))
	coreIv := np.MkInterval(0, np.Toffset(nCoresToRevoke))

	ctlFilePath := path.Join(np.PROCD, "~ip", np.RESOURCE_CTL)

	// Remove some cores from the procd.
	db.DPrintf("TEST", "Removing %v cores %v from procd.", nCoresToRevoke, coreIv)
	revokeMsg := resource.MakeResourceMsg(resource.Trequest, resource.Tcore, coreIv.String(), nCoresToRevoke)
	_, err = ts.SetFile(ctlFilePath, revokeMsg.Marshal(), np.OWRITE, 0)
	assert.Nil(t, err, "SetFile revoke: %v", err)

	// Run a proc which shouldn't fit on the newly resized procd.
	db.DPrintf("TEST", "Spawning a proc which shouldn't fit.")
	pid1 := proc.GenPid()
	spawnSleeperNcore(t, ts, pid1, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)

	time.Sleep(3 * SLEEP_MSECS)
	// Proc should not have run.
	checkSleeperResultFalse(t, ts, pid1)

	pid2 := proc.GenPid()
	db.DPrintf("TEST", "Spawning a proc which should fit.")
	spawnSleeperNcore(t, ts, pid2, proc.Tcore(linuxsched.NCores/2-1), SLEEP_MSECS)
	status, err = ts.WaitExit(pid2)
	assert.Nil(t, err, "WaitExit 2")
	assert.True(t, status.IsStatusOK(), "WaitExit status 2")
	checkSleeperResult(t, ts, pid2)
	db.DPrintf("TEST", "Proc which should fit ran")

	// Grant the procd back its cores.
	db.DPrintf("TEST", "Granting %v cores %v to procd.", nCoresToRevoke, coreIv)
	grantMsg := resource.MakeResourceMsg(resource.Tgrant, resource.Tcore, coreIv.String(), nCoresToRevoke)
	_, err = ts.SetFile(ctlFilePath, grantMsg.Marshal(), np.OWRITE, 0)
	assert.Nil(t, err, "SetFile grant: %v", err)

	// Make sure the proc ran.
	status, err = ts.WaitExit(pid1)
	assert.Nil(t, err, "WaitExit 3")
	assert.True(t, status.IsStatusOK(), "WaitExit status 3")
	checkSleeperResult(t, ts, pid1)

	ts.Shutdown()
}

func TestProcdResizeN(t *testing.T) {
	ts := test.MakeTstateAll(t)

	N := 5

	nCoresToRevoke := int(math.Ceil(float64(linuxsched.NCores)/2 + 1))
	coreIv := np.MkInterval(0, np.Toffset(nCoresToRevoke))

	ctlFilePath := path.Join(np.PROCD, "~ip", np.RESOURCE_CTL)
	for i := 0; i < N; i++ {
		db.DPrintf("TEST", "Resize i=%v", i)
		// Run a proc that claims all cores.
		pid := proc.GenPid()
		spawnSleeperNcore(t, ts, pid, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)
		status, err := ts.WaitExit(pid)
		assert.Nil(t, err, "WaitExit")
		assert.True(t, status.IsStatusOK(), "WaitExit status")
		checkSleeperResult(t, ts, pid)

		// Remove some cores from the procd.
		db.DPrintf("TEST", "Removing %v cores %v from procd.", nCoresToRevoke, coreIv)
		revokeMsg := resource.MakeResourceMsg(resource.Trequest, resource.Tcore, coreIv.String(), nCoresToRevoke)
		_, err = ts.SetFile(ctlFilePath, revokeMsg.Marshal(), np.OWRITE, 0)
		assert.Nil(t, err, "SetFile revoke: %v", err)

		// Run a proc which shouldn't fit on the newly resized procd.
		db.DPrintf("TEST", "Spawning a proc which shouldn't fit.")
		pid1 := proc.GenPid()
		spawnSleeperNcore(t, ts, pid1, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)

		time.Sleep(3 * SLEEP_MSECS)
		// Proc should not have run.
		checkSleeperResultFalse(t, ts, pid1)

		pid2 := proc.GenPid()
		db.DPrintf("TEST", "Spawning a proc which should fit.")
		spawnSleeperNcore(t, ts, pid2, proc.Tcore(linuxsched.NCores/2-1), SLEEP_MSECS)
		status, err = ts.WaitExit(pid2)
		assert.Nil(t, err, "WaitExit 2")
		assert.True(t, status.IsStatusOK(), "WaitExit status 2")
		checkSleeperResult(t, ts, pid2)
		db.DPrintf("TEST", "Proc which should fit ran")

		// Grant the procd back its cores.
		db.DPrintf("TEST", "Granting %v cores %v to procd.", nCoresToRevoke, coreIv)
		grantMsg := resource.MakeResourceMsg(resource.Tgrant, resource.Tcore, coreIv.String(), nCoresToRevoke)
		_, err = ts.SetFile(ctlFilePath, grantMsg.Marshal(), np.OWRITE, 0)
		assert.Nil(t, err, "SetFile grant: %v", err)

		// Make sure the proc ran.
		status, err = ts.WaitExit(pid1)
		assert.Nil(t, err, "WaitExit 3")
		assert.True(t, status.IsStatusOK(), "WaitExit status 3")
		checkSleeperResult(t, ts, pid1)
	}

	ts.Shutdown()
}

func TestProcdResizeEvict(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Run a proc that claims all cores save one.
	pid := spawnSpinnerNcore(t, ts, proc.Tcore(linuxsched.NCores)-1)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart")

	// Run a proc that claims one core.
	pid1 := spawnSpinnerNcore(t, ts, proc.Tcore(1))
	err = ts.WaitStart(pid1)
	assert.Nil(t, err, "WaitStart")

	// Revoke half + 1 of the procd's cores.
	nCoresToRevoke := int(math.Ceil(float64(linuxsched.NCores)/2 + 1))
	coreIv := np.MkInterval(0, np.Toffset(nCoresToRevoke))

	ctlFilePath := path.Join(np.PROCD, "~ip", np.RESOURCE_CTL)

	// Remove some cores from the procd.
	db.DPrintf("TEST", "Removing %v cores %v from procd.", nCoresToRevoke, coreIv)
	revokeMsg := resource.MakeResourceMsg(resource.Trequest, resource.Tcore, coreIv.String(), nCoresToRevoke)
	_, err = ts.SetFile(ctlFilePath, revokeMsg.Marshal(), np.OWRITE, 0)
	assert.Nil(t, err, "SetFile revoke: %v", err)

	db.DPrintf("TEST", "Waiting for small proc to exit")
	// Ensure that the small proc was evicted.
	status, err := ts.WaitExit(pid1)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusEvicted(), "WaitExit status")

	db.DPrintf("TEST", "Small proc exited.")

	// Evict the big proc
	err = ts.Evict(pid)
	assert.Nil(ts.T, err, "Evict")

	db.DPrintf("TEST", "Waiting for big proc to exit.")

	status, err = ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusEvicted(), "WaitExit status")

	db.DPrintf("TEST", "Big proc exited.")

	ts.Shutdown()
}

func TestProcdResizeAccurateStats(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Spawn NCores/2 spinners, each claiming two cores.
	pids := []proc.Tpid{}
	for i := 0; i < int(linuxsched.NCores)/2; i++ {
		pid := spawnSpinnerNcore(t, ts, proc.Tcore(2))
		err := ts.WaitStart(pid)
		assert.Nil(t, err, "WaitStart")
		pids = append(pids, pid)
	}

	// Revoke half of the procd's cores.
	nCoresToRevoke := int(math.Ceil(float64(linuxsched.NCores) / 2))
	coreIv := np.MkInterval(0, np.Toffset(nCoresToRevoke))

	ctlFilePath := path.Join(np.PROCD, "~ip", np.RESOURCE_CTL)

	// Remove some cores from the procd.
	db.DPrintf("TEST", "Removing %v cores %v from procd.", nCoresToRevoke, coreIv)
	revokeMsg := resource.MakeResourceMsg(resource.Trequest, resource.Tcore, coreIv.String(), nCoresToRevoke)
	_, err := ts.SetFile(ctlFilePath, revokeMsg.Marshal(), np.OWRITE, 0)
	assert.Nil(t, err, "SetFile revoke: %v", err)

	// Sleep for a bit
	time.Sleep(SLEEP_MSECS * time.Millisecond)

	// Get the procd's utilization.
	st := stats.StatInfo{}
	err = ts.GetFileJson(path.Join(np.PROCD, "~ip", np.STATSD), &st)
	assert.Nil(t, err, "statsd: %v", err)

	// Ensure that the procd is accurately representing the utilization (it
	// should show ~100% CPU utilization, not 50%).
	db.DPrintf("TEST", "Stats after shrink: %v", st)
	assert.True(t, st.Util > 90.0, "Util too low, %v < 90", st.Util)

	// Grant the procd back its cores.
	db.DPrintf("TEST", "Granting %v cores %v to procd.", nCoresToRevoke, coreIv)
	grantMsg := resource.MakeResourceMsg(resource.Tgrant, resource.Tcore, coreIv.String(), nCoresToRevoke)
	_, err = ts.SetFile(ctlFilePath, grantMsg.Marshal(), np.OWRITE, 0)
	assert.Nil(t, err, "SetFile grant: %v", err)

	// Sleep for a bit
	time.Sleep(SLEEP_MSECS * time.Millisecond)

	// Get the procd's utilization again.
	err = ts.GetFileJson(path.Join(np.PROCD, "~ip", np.STATSD), &st)
	assert.Nil(t, err, "statsd: %v", err)

	// Ensure that the procd's utilization has been adjusted again (it
	// should show ~50% CPU utilization, not 100%).
	db.DPrintf("TEST", "Stats after shrink: %v", st)
	assert.True(t, st.Util < 60.0, "Util too high, %v > 60", st.Util)

	// Evict all of the spinning procs.
	for _, pid := range pids {
		err := ts.Evict(pid)
		assert.Nil(ts.T, err, "Evict")
		status, err := ts.WaitExit(pid)
		assert.Nil(t, err, "WaitExit")
		assert.True(t, status.IsStatusEvicted(), "WaitExit status")
	}

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
		db.DPrintf("TEST", "Core %v utilization: %v", c, avgCoreUtil)
		if avgCoreUtil > 50.0 {
			coreOccupied = true
		}
	}
	return coreOccupied
}

func TestProcdResizeCoreRepinning(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Spawn NCores/2 spinners, each claiming two cores.
	pids := []proc.Tpid{}
	for i := 0; i < int(linuxsched.NCores)/2; i++ {
		pid := spawnSpinnerNcore(t, ts, proc.Tcore(2))
		err := ts.WaitStart(pid)
		assert.Nil(t, err, "WaitStart")
		pids = append(pids, pid)
	}

	// Revoke half of the procd's cores.
	nCoresToRevoke := int(math.Ceil(float64(linuxsched.NCores) / 2))
	coreIv := np.MkInterval(0, np.Toffset(nCoresToRevoke))

	ctlFilePath := path.Join(np.PROCD, "~ip", np.RESOURCE_CTL)

	// Create a map to sample core utilization levels on the cores which will be
	// revoked.
	coresMaps := []map[string]bool{}
	for i := coreIv.Start; i < coreIv.End; i++ {
		coresMaps = append(coresMaps, map[string]bool{"cpu" + strconv.Itoa(int(i)): true})
	}

	coreOccupied := anyCoresOccupied(coresMaps)
	// Make sure that at least some of the cores to be revoked has a spinning
	// proc on it.
	assert.True(t, coreOccupied, "No cores occupied")

	// Remove some cores from the procd.
	db.DPrintf("TEST", "Removing %v cores %v from procd.", nCoresToRevoke, coreIv)
	revokeMsg := resource.MakeResourceMsg(resource.Trequest, resource.Tcore, coreIv.String(), nCoresToRevoke)
	_, err := ts.SetFile(ctlFilePath, revokeMsg.Marshal(), np.OWRITE, 0)
	assert.Nil(t, err, "SetFile revoke: %v", err)

	// Sleep for a bit
	time.Sleep(SLEEP_MSECS * time.Millisecond)

	coreOccupied = anyCoresOccupied(coresMaps)
	// Ensure that none of the revoked cores have spinning procs running on them.
	assert.False(t, coreOccupied, "Core still occupied")

	// Evict all of the spinning procs.
	for _, pid := range pids {
		err := ts.Evict(pid)
		assert.Nil(ts.T, err, "Evict")
		status, err := ts.WaitExit(pid)
		assert.Nil(t, err, "WaitExit")
		assert.True(t, status.IsStatusEvicted(), "WaitExit status")
	}

	ts.Shutdown()
}
