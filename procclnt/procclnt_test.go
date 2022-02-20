package procclnt_test

import (
	"fmt"
	"log"
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
	"ulambda/proc"
	"ulambda/test"
)

const (
	SLEEP_MSECS = 2000
)

const program = "procclnt_test"

func procd(ts *test.Tstate) string {
	st, err := ts.ReadDir("name/procd")
	assert.Nil(ts.T, err, "Readdir")
	return st[0].Name
}

func spawnSpinner(t *testing.T, ts *test.Tstate) string {
	pid := proc.GenPid()
	a := proc.MakeProcPid(pid, "bin/user/spinner", []string{"name/"})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return pid
}

func spawnSleeperWithPid(t *testing.T, ts *test.Tstate, pid string) {
	spawnSleeperNcore(t, ts, pid, 0, SLEEP_MSECS)
}

func spawnSleeper(t *testing.T, ts *test.Tstate) string {
	pid := proc.GenPid()
	spawnSleeperWithPid(t, ts, pid)
	return pid
}

func spawnSleeperNcore(t *testing.T, ts *test.Tstate, pid string, ncore proc.Tcore, msecs int) {
	a := proc.MakeProcPid(pid, "bin/user/sleeper", []string{fmt.Sprintf("%dms", msecs), "name/out_" + pid})
	a.Ncore = ncore
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DLPrintf("SCHEDD", "Spawn %v\n", a)
}

func spawnSpawner(t *testing.T, ts *test.Tstate, childPid string, msecs int) string {
	p := proc.MakeProc("bin/user/spawner", []string{"false", childPid, "bin/user/sleeper", fmt.Sprintf("%dms", msecs), "name/out_" + childPid})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	return p.Pid
}

func checkSleeperResult(t *testing.T, ts *test.Tstate, pid string) bool {
	res := true
	b, err := ts.GetFile("name/out_" + pid)
	res = assert.Nil(t, err, "GetFile") && res
	res = assert.Equal(t, string(b), "hello", "Output") && res

	return res
}

func checkSleeperResultFalse(t *testing.T, ts *test.Tstate, pid string) {
	b, err := ts.GetFile("name/out_" + pid)
	assert.NotNil(t, err, "GetFile")
	assert.NotEqual(t, string(b), "hello", "Output")
}

func TestWaitExitSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong")

	// cleaned up (may take a bit)
	time.Sleep(500 * time.Millisecond)
	_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid))
	assert.NotNil(t, err, "Stat %v", path.Join(proc.PIDS, pid))

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, pid)

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
	_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid))
	assert.NotNil(t, err, "Stat")

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
	_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid))
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
	sts, err := ts.ReadDir(path.Join("name/procd", procd(ts), np.PROCD_RUNNING))
	assert.Nil(t, err, "Readdir")
	assert.True(t, fslib.Present(sts, []string{pid}), "pid")

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

func TestCrashProc(t *testing.T) {
	ts := test.MakeTstateAll(t)

	a := proc.MakeProc("bin/user/crash", []string{})
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
	a := proc.MakeProc("bin/user/parentexit", []string{fmt.Sprintf("%dms", SLEEP_MSECS), pid1})
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
	b, err := ts.GetFile("name/out_" + pid1)
	assert.Nil(t, err, "GetFile")
	assert.Equal(t, string(b), "hello", "Output")

	// .. and cleaned up
	_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid1))
	assert.NotNil(t, err, "Stat")

	ts.Shutdown()
}

func TestEarlyExitN(t *testing.T) {
	ts := test.MakeTstateAll(t)
	nProcs := 500
	var done sync.WaitGroup
	done.Add(nProcs)

	for i := 0; i < nProcs; i++ {
		go func() {
			pid1 := proc.GenPid()
			a := proc.MakeProc("bin/user/parentexit", []string{fmt.Sprintf("%dms", 0), pid1})
			err := ts.Spawn(a)
			assert.Nil(t, err, "Spawn")

			// Wait for parent to finish
			status, err := ts.WaitExit(a.Pid)
			assert.Nil(t, err, "WaitExit")
			assert.True(t, status.IsStatusOK(), "WaitExit")

			time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

			// Child should have exited
			b, err := ts.GetFile("name/out_" + pid1)
			assert.Nil(t, err, "GetFile")
			assert.Equal(t, string(b), "hello", "Output")

			// .. and cleaned up
			_, err = ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid1))
			assert.NotNil(t, err, "Stat")
			done.Done()
		}()
	}
	done.Wait()

	ts.Shutdown()
}

// Spawn a bunch of procs concurrently, then wait for all of them & check
// their result
func TestConcurrentProcs(t *testing.T) {
	ts := test.MakeTstateAll(t)

	nProcs := 8
	pids := map[string]int{}

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
		go func(pid string, started *sync.WaitGroup, i int) {
			barrier.Done()
			barrier.Wait()
			spawnSleeperWithPid(t, ts, pid)
			started.Done()
		}(pid, &started, i)
	}

	started.Wait()

	for pid, i := range pids {
		_ = i
		go func(pid string, done *sync.WaitGroup, i int) {
			defer done.Done()
			ts.WaitExit(pid)
			checkSleeperResult(t, ts, pid)
			_, err := ts.Stat(path.Join(np.PROCD, "~ip", proc.PIDS, pid))
			assert.NotNil(t, err, "Stat")
		}(pid, &done, i)
	}

	done.Wait()

	ts.Shutdown()
}

func evict(ts *test.Tstate, pid string) {
	time.Sleep(SLEEP_MSECS / 2 * time.Millisecond)
	err := ts.Evict(pid)
	assert.Nil(ts.T, err, "evict")
}

func TestEvict(t *testing.T) {
	ts := test.MakeTstateAll(t)

	start := time.Now()
	pid := spawnSleeper(t, ts)

	go evict(ts, pid)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status.IsStatusEvicted(), "WaitExit status")
	end := time.Now()

	assert.True(t, end.Sub(start) < SLEEP_MSECS*time.Millisecond, "Didn't evict early enough.")
	assert.True(t, end.Sub(start) > (SLEEP_MSECS/2)*time.Millisecond, "Evicted too early")

	// Make sure the proc didn't finish
	checkSleeperResultFalse(t, ts, pid)

	ts.Shutdown()
}

func testFencer(t *testing.T, part string) {
	const (
		N         = 20
		FENCE_DIR = "name/fence"
	)

	ts := test.MakeTstateAll(t)
	pids := []string{}

	// XXX use the same dir independent of machine running proc
	dir := "name/ux/~ip/outdir"
	ts.RmDir(dir)
	err := ts.Mkdir(dir, 0777)
	err = ts.Mkdir(FENCE_DIR, 0777)
	assert.Nil(t, err, "mkdir error")
	_, err = ts.PutFile(FENCE_DIR+"/cnt", 0777, np.OWRITE, []byte(strconv.Itoa(0)))
	assert.Nil(t, err, "makefile error")

	_, err = ts.PutFile(dir+"/A", 0777, np.OWRITE, []byte(strconv.Itoa(0)))
	assert.Nil(t, err, "makefile error")

	for i := 0; i < N; i++ {
		a := proc.MakeProc("bin/user/fencer", []string{part, FENCE_DIR, dir})
		err = ts.Spawn(a)
		assert.Nil(t, err, "Spawn")
		pids = append(pids, a.Pid)
	}

	for _, pid := range pids {
		status, err := ts.WaitExit(pid)
		log.Printf("status %v\n", status)
		assert.True(t, err != nil || !status.IsStatusErr() || status.Msg() != "Invariant violated", "Exit status wrong")
	}
	ts.Shutdown()
}

func TestFencerNoPart(t *testing.T) {
	testFencer(t, "NO")
}

func TestFencerWithPart(t *testing.T) {
	testFencer(t, "YES")
}

func TestReserveCores(t *testing.T) {
	ts := test.MakeTstateAll(t)

	linuxsched.ScanTopology()

	start := time.Now()
	pid := proc.GenPid()
	spawnSleeperNcore(t, ts, pid, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)

	pid1 := proc.GenPid()
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

	linuxsched.ScanTopology()

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

	assert.True(t, end.Sub(start) < (SLEEP_MSECS*2)*time.Millisecond, "Parallelized")

	ts.Shutdown()
}

func TestEvictN(t *testing.T) {
	ts := test.MakeTstateAll(t)

	linuxsched.ScanTopology()
	N := int(linuxsched.NCores)

	pids := []string{}
	for i := 0; i < N; i++ {
		pid := spawnSpinner(t, ts)
		pids = append(pids, pid)
		go evict(ts, pid)
	}

	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)
	for i := 0; i < N; i++ {
		status, err := ts.WaitExit(pids[i])
		assert.Nil(t, err, "WaitExit")
		assert.True(t, status.IsStatusEvicted(), "WaitExit status")
	}

	ts.Shutdown()
}

func getNChildren(ts *test.Tstate) int {
	c, err := ts.GetChildren(proc.GetProcDir())
	assert.Nil(ts.T, err, "getnchildren")
	return len(c)
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

	err = ts.Mkdir(OUTDIR, 0777)
	assert.Nil(t, err, "Mkdir")

	// Start a bunch of replicated spinner procs.
	sm := groupmgr.Start(ts.FsLib, ts.ProcClnt, N_REPL, "bin/user/spinner", []string{OUTDIR}, 0)
	nChildren += N_REPL

	// Wait for them to spawn.
	time.Sleep(1 * time.Second)

	// Make sure they spawned correctly.
	st, err := ts.ReadDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #1")
	assert.Equal(t, nChildren, getNChildren(ts), "wrong num children")

	err = ts.KillOne(np.PROCD)
	assert.Nil(t, err, "kill procd")

	// Wait for them to respawn.
	time.Sleep(1 * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.ReadDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #3")

	err = ts.KillOne(np.PROCD)
	assert.Nil(t, err, "kill procd")

	// Wait for them to respawn.
	time.Sleep(1 * time.Second)

	// Make sure they spawned correctly.
	st, err = ts.ReadDir(OUTDIR)
	assert.Nil(t, err, "readdir1")
	assert.Equal(t, N_REPL, len(st), "wrong num spinners check #2")

	sm.Stop()

	ts.Shutdown()
}
