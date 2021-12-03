package procclnt_test

import (
	"fmt"
	"path"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/linuxsched"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	SLEEP_MSECS = 2000
)

type Tstate struct {
	*procclnt.ProcClnt
	*fslib.FsLib
	t *testing.T
	s *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	ts.s = kernel.MakeSystemAll(bin)
	ts.FsLib = fslib.MakeFsLibAddr("procclnt_test", fslib.Named())
	ts.ProcClnt = procclnt.MakeProcClntInit(ts.FsLib, fslib.Named())
	ts.t = t
	return ts
}

func (ts *Tstate) procd(t *testing.T) string {
	st, err := ts.ReadDir("name/procd")
	assert.Nil(t, err, "Readdir")
	return st[0].Name
}

func makeTstateNoBoot(t *testing.T, pid string) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.FsLib = fslib.MakeFsLibAddr("procclnt_test", fslib.Named())
	ts.ProcClnt = procclnt.MakeProcClntInit(ts.FsLib, fslib.Named())
	return ts
}

func spawnSpinner(t *testing.T, ts *Tstate) string {
	pid := proc.GenPid()
	a := proc.MakeProcPid(pid, "bin/user/spinner", []string{"name/out_" + pid})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return pid
}

func spawnSleeperWithPid(t *testing.T, ts *Tstate, pid string) {
	spawnSleeperNcore(t, ts, pid, 0, SLEEP_MSECS)
}

func spawnSleeper(t *testing.T, ts *Tstate) string {
	pid := proc.GenPid()
	spawnSleeperWithPid(t, ts, pid)
	return pid
}

func spawnSleeperNcore(t *testing.T, ts *Tstate, pid string, ncore proc.Tcore, msecs int) {
	a := proc.MakeProcPid(pid, "bin/user/sleeper", []string{fmt.Sprintf("%dms", msecs), "name/out_" + pid})
	a.Ncore = ncore
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DLPrintf("SCHEDD", "Spawn %v\n", a)
}

func checkSleeperResult(t *testing.T, ts *Tstate, pid string) bool {
	res := true
	b, err := ts.ReadFile("name/out_" + pid)
	res = assert.Nil(t, err, "ReadFile") && res
	res = assert.Equal(t, string(b), "hello", "Output") && res

	return res
}

func checkSleeperResultFalse(t *testing.T, ts *Tstate, pid string) {
	b, err := ts.ReadFile("name/out_" + pid)
	assert.NotNil(t, err, "ReadFile")
	assert.NotEqual(t, string(b), "hello", "Output")
}

func TestWaitExit(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.Equal(t, "OK", status, "Exit status wrong")

	// cleaned up
	_, err = ts.Stat(proc.PidDir(pid))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, pid)

	ts.s.Shutdown()
}

func TestWaitExitParentRetStat(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)
	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.Equal(t, "OK", status, "Exit status wrong")

	// cleaned up
	_, err = ts.Stat(proc.PidDir(pid))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(t, ts, pid)

	ts.s.Shutdown()
}

func TestWaitStart(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()

	pid := spawnSleeper(t, ts)
	err := ts.WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")

	end := time.Now()

	assert.True(t, end.Sub(start) < SLEEP_MSECS*time.Millisecond, "WaitStart waited too long")

	// Check if proc exists
	sts, err := ts.ReadDir(path.Join("name/procd", ts.procd(t), named.PROCD_RUNNING))
	assert.Nil(t, err, "Readdir")

	// skip ctl entry
	i := 0
	if sts[i].Name == "ctl" {
		i = 1
	}
	assert.Equal(t, pid, sts[i].Name, "pid")

	// Make sure the proc hasn't finished yet...
	checkSleeperResultFalse(t, ts, pid)

	ts.WaitExit(pid)

	checkSleeperResult(t, ts, pid)

	ts.s.Shutdown()
}

// Should exit immediately
func TestWaitNonexistentProc(t *testing.T) {
	ts := makeTstate(t)

	ch := make(chan bool)

	pid := proc.GenPid()
	go func() {
		ts.WaitExit(pid)
		ch <- true
	}()

	done := <-ch
	assert.True(t, done, "Nonexistent proc")

	close(ch)

	ts.s.Shutdown()
}

func TestCrashProc(t *testing.T) {
	ts := makeTstate(t)

	a := proc.MakeProc("bin/user/crash", []string{})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	err = ts.WaitStart(a.Pid)
	assert.Nil(t, err, "WaitStart error")

	status, err := ts.WaitExit(a.Pid)
	assert.Nil(t, err, "WaitExit")
	assert.Equal(t, "exit status 2", status, "WaitExit")

	ts.s.Shutdown()
}

func TestFailSpawn(t *testing.T) {
	ts := makeTstate(t)

	a := proc.MakeEmptyProc()
	a.Pid = proc.GenPid()
	err := ts.Spawn(a)
	assert.NotNil(t, err, "Spawn")

	// child should not exist
	_, err = ts.Stat(proc.PidDir(a.Pid))
	assert.NotNil(t, err, "Stat")

	ts.s.Shutdown()
}

func TestEarlyExit1(t *testing.T) {
	ts := makeTstate(t)

	pid1 := proc.GenPid()
	a := proc.MakeProc("bin/user/parentexit", []string{fmt.Sprintf("%dms", SLEEP_MSECS), pid1})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")

	// Wait for parent to finish
	status, err := ts.WaitExit(a.Pid)
	assert.Nil(t, err, "WaitExit")
	assert.Equal(t, "OK", status, "WaitExit")

	// Child should be still running
	_, err = ts.Stat(proc.PidDir(pid1))
	assert.Nil(t, err, "Stat")

	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

	// Child should have exited
	b, err := ts.ReadFile("name/out_" + pid1)
	assert.Nil(t, err, "ReadFile")
	assert.Equal(t, string(b), "hello", "Output")

	// .. and cleaned up
	_, err = ts.Stat(proc.PidDir(pid1))
	assert.NotNil(t, err, "Stat")

	ts.s.Shutdown()
}

func TestEarlyExitN(t *testing.T) {
	ts := makeTstate(t)
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
			assert.Equal(t, "OK", status, "WaitExit")

			time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

			// Child should have exited
			b, err := ts.ReadFile("name/out_" + pid1)
			assert.Nil(t, err, "ReadFile")
			assert.Equal(t, string(b), "hello", "Output")

			// .. and cleaned up
			_, err = ts.Stat(proc.PidDir(pid1))
			assert.NotNil(t, err, "Stat")
			done.Done()
		}()
	}
	done.Wait()

	ts.s.Shutdown()
}

// Spawn a bunch of procs concurrently, then wait for all of them & check
// their result
func TestConcurrentProcs(t *testing.T) {
	ts := makeTstate(t)

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
			_, err := ts.Stat(proc.PidDir(pid))
			assert.NotNil(t, err, "Stat")
		}(pid, &done, i)
	}

	done.Wait()

	ts.s.Shutdown()
}

func (ts *Tstate) evict(pid string) {
	time.Sleep(SLEEP_MSECS / 2 * time.Millisecond)
	err := ts.Evict(pid)
	assert.Nil(ts.t, err, "evict")
}

func TestEvict(t *testing.T) {
	ts := makeTstate(t)

	start := time.Now()
	pid := spawnSleeper(t, ts)

	go ts.evict(pid)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.Equal(t, "EVICTED", status, "WaitExit status")
	end := time.Now()

	assert.True(t, end.Sub(start) < SLEEP_MSECS*time.Millisecond, "Didn't evict early enough.")
	assert.True(t, end.Sub(start) > (SLEEP_MSECS/2)*time.Millisecond, "Evicted too early")

	// Make sure the proc didn't finish
	checkSleeperResultFalse(t, ts, pid)

	ts.s.Shutdown()
}

func testLocker(t *testing.T, part string) {
	const N = 20

	ts := makeTstate(t)
	pids := []string{}

	// XXX use the same dir independent of machine running proc
	dir := "name/ux/~ip/outdir"
	ts.RmDir(dir)
	err := ts.Mkdir(dir, 0777)
	err = ts.Mkdir("name/locktest", 0777)
	assert.Nil(t, err, "mkdir error")
	err = ts.MakeFile("name/locktest/cnt", 0777, np.OWRITE, []byte(strconv.Itoa(0)))
	assert.Nil(t, err, "makefile error")
	err = ts.MakeFile(dir+"/A", 0777, np.OWRITE, []byte(strconv.Itoa(0)))
	assert.Nil(t, err, "makefile error")

	for i := 0; i < N; i++ {
		a := proc.MakeProc("bin/user/locker", []string{part, dir})
		err = ts.Spawn(a)
		assert.Nil(t, err, "Spawn")
		pids = append(pids, a.Pid)
	}

	for _, pid := range pids {
		status, _ := ts.WaitExit(pid)
		assert.NotEqual(t, "Invariant violated", status, "Exit status wrong")
	}
	ts.s.Shutdown()
}

func TestLockerNoPart(t *testing.T) {
	testLocker(t, "NO")
}

//func TestLockerWithPart(t *testing.T) {
//	testLocker(t, "YES")
//}

func TestReserveCores(t *testing.T) {
	ts := makeTstate(t)

	linuxsched.ScanTopology()

	start := time.Now()
	pid := proc.GenPid()
	spawnSleeperNcore(t, ts, pid, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)

	pid1 := proc.GenPid()
	spawnSleeperNcore(t, ts, pid1, 1, SLEEP_MSECS)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.Equal(t, "OK", status, "WaitExit status")

	// Make sure the second proc didn't finish
	checkSleeperResult(t, ts, pid)
	checkSleeperResultFalse(t, ts, pid1)

	status, err = ts.WaitExit(pid1)
	assert.Nil(t, err, "WaitExit 2")
	assert.Equal(t, "OK", status, "WaitExit status 2")
	end := time.Now()

	assert.True(t, end.Sub(start) > (SLEEP_MSECS*2)*time.Millisecond, "Parallelized")

	ts.s.Shutdown()
}

func TestWorkStealing(t *testing.T) {
	ts := makeTstate(t)

	ts.s.BootProcd()

	linuxsched.ScanTopology()

	start := time.Now()
	pid := proc.GenPid()
	spawnSleeperNcore(t, ts, pid, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)

	pid1 := proc.GenPid()
	spawnSleeperNcore(t, ts, pid1, proc.Tcore(linuxsched.NCores), SLEEP_MSECS)

	status, err := ts.WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.Equal(t, "OK", status, "WaitExit status")

	status, err = ts.WaitExit(pid1)
	assert.Nil(t, err, "WaitExit 2")
	assert.Equal(t, "OK", status, "WaitExit status 2")
	end := time.Now()

	// Make sure both procs finished
	checkSleeperResult(t, ts, pid)
	checkSleeperResult(t, ts, pid1)

	assert.True(t, end.Sub(start) < (SLEEP_MSECS*2)*time.Millisecond, "Parallelized")

	ts.s.Shutdown()
}

func TestEvictN(t *testing.T) {
	ts := makeTstate(t)

	linuxsched.ScanTopology()
	N := int(linuxsched.NCores)

	pids := []string{}
	for i := 0; i < N; i++ {
		pid := spawnSpinner(t, ts)
		pids = append(pids, pid)
		go ts.evict(pid)
	}

	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)
	for i := 0; i < N; i++ {
		status, err := ts.WaitExit(pids[i])
		assert.Nil(t, err, "WaitExit")
		assert.Equal(t, "EVICTED", status, "WaitExit status")
	}

	ts.s.Shutdown()
}
