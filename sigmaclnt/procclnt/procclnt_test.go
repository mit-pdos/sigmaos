package procclnt_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
	linuxsched "sigmaos/util/linux/sched"
)

const (
	SLEEP_MSECS           = 2000
	CRASH_MSECS           = 5
	NPROC                 = "3000"
	NPROC1                = "1000"
	BURST                 = "20"
	N_NODES               = 2
	REALM1      sp.Trealm = "testrealm1"
)

const program = "procclnt_test"

func msched(ts *test.RealmTstate) string {
	st, err := ts.GetDir(sp.MSCHED)
	assert.Nil(ts.Ts.T, err, "Readdir")
	return st[0].Name
}

func spawnSpinner(ts *test.RealmTstate) sp.Tpid {
	return spawnSpinnerMcpu(ts, 0)
}

func spawnSpinnerMcpu(ts *test.RealmTstate, mcpu proc.Tmcpu) sp.Tpid {
	pid := sp.GenPid("spinner")
	a := proc.NewProcPid(pid, "spinner", []string{"name/"})
	a.SetMcpu(mcpu)
	err := ts.Spawn(a)
	assert.Nil(ts.Ts.T, err, "Spawn")
	return pid
}

// Make a burst of LC procs
func burstSpawnSpinner(ts *test.RealmTstate, N uint) []*proc.Proc {
	ps := make([]*proc.Proc, 0, N)
	for i := uint(0); i < N; i++ {
		p := proc.NewProc("spinner", []string{"name/"})
		p.SetMcpu(1000)
		err := ts.Spawn(p)
		assert.Nil(ts.Ts.T, err, "Failed spawning some procs: %v", err)
		ps = append(ps, p)
	}
	return ps
}

func spawnSleeperWithPid(ts *test.RealmTstate, pid sp.Tpid) {
	spawnSleeperMcpu(ts, pid, 0, SLEEP_MSECS)
}

func spawnSleeper(ts *test.RealmTstate) sp.Tpid {
	pid := sp.GenPid("sleeper")
	spawnSleeperWithPid(ts, pid)
	return pid
}

func spawnSleeperMcpu(ts *test.RealmTstate, pid sp.Tpid, mcpu proc.Tmcpu, msecs int) {
	a := proc.NewProcPid(pid, "sleeper", []string{fmt.Sprintf("%dms", msecs), "name/"})
	a.SetMcpu(mcpu)
	err := ts.Spawn(a)
	assert.Nil(ts.Ts.T, err, "Spawn")
}

func spawnSpawner(ts *test.RealmTstate, wait bool, childPid sp.Tpid, msecs int, em *crash.TeventMap) sp.Tpid {
	p := proc.NewProc("spawner", []string{strconv.FormatBool(wait), childPid.String(), "sleeper", fmt.Sprintf("%dms", msecs), "name/"})
	err := em.AppendEnv(p)
	assert.Nil(ts.Ts.T, err)
	err = ts.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Spawn")
	return p.GetPid()
}

func checkSleeperResult(ts *test.RealmTstate, pid sp.Tpid) bool {
	res := true
	b, err := ts.GetFile("name/" + pid.String() + "_out")
	res = assert.Nil(ts.Ts.T, err, "GetFile err: %v", err) && res
	res = assert.Equal(ts.Ts.T, "hello", string(b), "Output") && res

	return res
}

func checkSleeperResultFalse(ts *test.RealmTstate, pid sp.Tpid) {
	b, err := ts.GetFile("name/" + pid.String() + "_out")
	assert.NotNil(ts.Ts.T, err, "GetFile")
	assert.NotEqual(ts.Ts.T, string(b), "hello", "Output")
}

func cleanSleeperResult(ts *test.RealmTstate, pid sp.Tpid) {
	ts.SigmaClnt.Remove("name/" + pid.String() + "_out")
}

func spawnWaitSleeper(ts *test.RealmTstate, kernels []string) *proc.Proc {
	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	if kernels != nil {
		a.SetKernels(kernels)
	}
	err := ts.Spawn(a)
	assert.Nil(ts.Ts.T, err, "Spawn")

	status, err := ts.WaitExit(a.GetPid())
	assert.Nil(ts.Ts.T, err, "WaitExit error")
	assert.True(ts.Ts.T, status != nil && status.IsStatusOK(), "Exit status wrong: %v", status)

	cleanSleeperResult(ts, a.GetPid())
	return a
}

func TestCompile(t *testing.T) {
}

func TestErrStringCrashed(t *testing.T) {
	msg := `"{Err: "Non-sigma error" Obj: "" (exit status 5)}`
	err := serr.NewErrString(msg)
	assert.True(t, err.ErrCode == serr.TErrError)
	assert.Equal(t, err.Err.Error(), proc.CRASHSTATUS)
}

func TestWaitExitSimpleSingleBE(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	spawnWaitSleeper(mrts.GetRealm(test.REALM1), nil)
}

func TestWaitExitSimpleSingleLC(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	a.SetMcpu(1000)
	db.DPrintf(db.TEST, "Pre spawn")
	err := mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong: %v", status)

	cleanSleeperResult(mrts.GetRealm(test.REALM1), a.GetPid())
}

func TestWaitExitOne(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	start := time.Now()

	pid := spawnSleeper(mrts.GetRealm(test.REALM1))
	status, err := mrts.GetRealm(test.REALM1).WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong")

	// cleaned up (may take a bit)
	time.Sleep(500 * time.Millisecond)
	_, err = mrts.GetRealm(test.REALM1).Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat %v", filepath.Join(sp.PIDS, pid.String()))

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(mrts.GetRealm(test.REALM1), pid)

	cleanSleeperResult(mrts.GetRealm(test.REALM1), pid)
}

func TestWaitExitN(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	nProcs := 100
	var done sync.WaitGroup
	done.Add(nProcs)

	for i := 0; i < nProcs; i++ {
		go func() {
			pid := spawnSleeper(mrts.GetRealm(test.REALM1))
			status, err := mrts.GetRealm(test.REALM1).WaitExit(pid)
			assert.Nil(t, err, "WaitExit error")
			assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong %v", status)
			db.DPrintf(db.TEST, "Exited %v", pid)

			checkSleeperResult(mrts.GetRealm(test.REALM1), pid)
			cleanSleeperResult(mrts.GetRealm(test.REALM1), pid)

			done.Done()
		}()
	}
	done.Wait()
}

func TestWaitExitParentRetStat(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	start := time.Now()

	pid := spawnSleeper(mrts.GetRealm(test.REALM1))
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)
	status, err := mrts.GetRealm(test.REALM1).WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil && status.IsStatusOK(), "Exit status wrong")

	// cleaned up
	for {
		_, err = mrts.GetRealm(test.REALM1).Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
		if err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
		db.DPrintf(db.TEST, "PID dir not deleted yet.")
	}
	assert.NotNil(t, err, "Stat %v", filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)

	checkSleeperResult(mrts.GetRealm(test.REALM1), pid)
	cleanSleeperResult(mrts.GetRealm(test.REALM1), pid)
}

func TestWaitExitParentAbandons(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	start := time.Now()

	cPid := sp.GenPid("sleeper")
	pid := spawnSpawner(mrts.GetRealm(test.REALM1), false, cPid, SLEEP_MSECS, nil)
	err := mrts.GetRealm(test.REALM1).WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(pid)
	assert.True(t, status != nil && status.IsStatusOK(), "WaitExit status error")
	assert.Nil(t, err, "WaitExit error")
	// Wait for the child to run & finish
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

	// cleaned up
	_, err = mrts.GetRealm(test.REALM1).Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)
}

func TestWaitExitParentCrash(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	start := time.Now()

	e0 := crash.NewEvent(crash.SPAWNER_CRASH, CRASH_MSECS, 0.6)
	em := crash.NewTeventMapOne(e0)
	e1 := crash.NewEvent(crash.SPAWNER_PARTITION, CRASH_MSECS, 0.6)
	em.Insert(e1)

	cPid := sp.GenPid("spawner")
	pid := spawnSpawner(mrts.GetRealm(test.REALM1), true, cPid, SLEEP_MSECS, em)
	err := mrts.GetRealm(test.REALM1).WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(pid)
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status != nil)
	assert.True(t, status.IsStatusErr())
	sr := serr.NewErrString(status.Msg())
	assert.Equal(t, sr.Err.Error(), proc.CRASHSTATUS)
	// Wait for the child to run & finish
	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

	// cleaned up
	_, err = mrts.GetRealm(test.REALM1).Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
	assert.NotNil(t, err, "Stat")

	end := time.Now()

	assert.True(t, end.Sub(start) > SLEEP_MSECS*time.Millisecond)
}

func TestWaitStart(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	pid := spawnSleeper(mrts.GetRealm(test.REALM1))
	err := mrts.GetRealm(test.REALM1).WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")

	// Make sure the proc hasn't finished yet...
	checkSleeperResultFalse(mrts.GetRealm(test.REALM1), pid)

	mrts.GetRealm(test.REALM1).WaitExit(pid)

	// Make sure the proc finished...
	checkSleeperResult(mrts.GetRealm(test.REALM1), pid)

	cleanSleeperResult(mrts.GetRealm(test.REALM1), pid)
}

// Should exit immediately
func TestWaitNonexistentProc(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ch := make(chan bool)

	pid := sp.GenPid("nonexistent")
	go func() {
		mrts.GetRealm(test.REALM1).WaitExit(pid)
		ch <- true
	}()

	done := <-ch
	assert.True(t, done, "Nonexistent proc")

	close(ch)
}

func TestCrashProcOne(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	a := proc.NewProc("crash", []string{})
	em := crash.NewTeventMapOne(crash.NewEvent(crash.CRASH_CRASH, 0, float64(1.0)))
	err := em.AppendEnv(a)
	assert.Nil(t, err)
	err = mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Spawn")

	err = mrts.GetRealm(test.REALM1).WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")

	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusErr(), "Status not err")
	sr := serr.NewErrString(status.Msg())
	assert.Equal(t, sr.Err.Error(), proc.CRASHSTATUS, "WaitExit")
}

func TestPartitionProcOne(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	a := proc.NewProc("crash", []string{})
	em := crash.NewTeventMapOne(crash.NewEvent(crash.CRASH_PARTITION, 0, float64(1.0)))
	err := em.AppendEnv(a)
	assert.Nil(t, err)
	err = mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Spawn")

	err = mrts.GetRealm(test.REALM1).WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")

	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusErr(), "Status not err")
	sr := serr.NewErrString(status.Msg())
	assert.Equal(t, sr.Err.Error(), proc.CRASHSTATUS, "WaitExit")
}

func TestEarlyExit1(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	pid1 := sp.GenPid("parentexit")
	a := proc.NewProc("parentexit", []string{fmt.Sprintf("%dms", SLEEP_MSECS), pid1.String()})
	err := mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Spawn")

	// Wait for parent to finish
	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusOK(), "WaitExit")

	// Child should not have terminated yet.
	checkSleeperResultFalse(mrts.GetRealm(test.REALM1), pid1)

	time.Sleep(2 * SLEEP_MSECS * time.Millisecond)

	// Child should have exited
	b, err := mrts.GetRealm(test.REALM1).GetFile("name/" + pid1.String() + "_out")
	assert.Nil(t, err, "GetFile")
	assert.Equal(t, "hello", string(b), "Output")

	// .. and cleaned up
	_, err = mrts.GetRealm(test.REALM1).Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid1.String()))
	assert.NotNil(t, err, "Stat")

	cleanSleeperResult(mrts.GetRealm(test.REALM1), pid1)
}

func TestEarlyExitN(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	nProcs := 50 // 500
	const MAX_RETRY = 10
	var done sync.WaitGroup
	done.Add(nProcs)

	for i := 0; i < nProcs; i++ {
		go func(i int) {
			pid1 := sp.GenPid("parentexit")
			a := proc.NewProc("parentexit", []string{fmt.Sprintf("%dms", 0), pid1.String()})
			err := mrts.GetRealm(test.REALM1).Spawn(a)
			assert.Nil(t, err, "Spawn")

			// Wait for parent to finish
			status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
			assert.Nil(t, err, "WaitExit err: %v", err)
			assert.True(t, status != nil && status.IsStatusOK(), "WaitExit: %v", status)

			var gotfile bool
			var contentsCorrect bool
			var b []byte
			var err2 error
			for i := 0; i < MAX_RETRY && (!gotfile || !contentsCorrect); i++ {
				b, err2 = mrts.GetRealm(test.REALM1).GetFile("name/" + pid1.String() + "_out")
				gotfile = gotfile || err2 == nil
				contentsCorrect = contentsCorrect || string(b) == "hello"
				time.Sleep(time.Second)
			}

			// Child should have exited
			assert.True(t, gotfile, "GetFile failed: %v", err2)
			assert.True(t, contentsCorrect, "Incorrect file contents: %v", string(b))

			// .. and cleaned up
			_, err = mrts.GetRealm(test.REALM1).Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid1.String()))
			assert.NotNil(t, err, "Stat")

			cleanSleeperResult(mrts.GetRealm(test.REALM1), pid1)

			done.Done()
		}(i)
	}
	done.Wait()
}

// Spawn a bunch of procs concurrently, then wait for all of them & check
// their result
func TestConcurrentProcs(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

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
			spawnSleeperWithPid(mrts.GetRealm(test.REALM1), pid)
			started.Done()
		}(pid, &started, i)
	}

	started.Wait()

	for pid, i := range pids {
		_ = i
		go func(pid sp.Tpid, done *sync.WaitGroup, i int) {
			defer done.Done()
			mrts.GetRealm(test.REALM1).WaitExit(pid)
			checkSleeperResult(mrts.GetRealm(test.REALM1), pid)
			cleanSleeperResult(mrts.GetRealm(test.REALM1), pid)
			time.Sleep(100 * time.Millisecond)
			_, err := mrts.GetRealm(test.REALM1).Stat(filepath.Join(sp.MSCHED, sp.LOCAL, sp.PIDS, pid.String()))
			assert.NotNil(t, err, "Stat %v", filepath.Join(sp.PIDS, pid.String()))
		}(pid, &done, i)
	}

	done.Wait()
}

func evict(ts *test.RealmTstate, pid sp.Tpid) {
	err := ts.WaitStart(pid)
	assert.Nil(ts.Ts.T, err, "Wait start err %v", err)
	err = ts.Evict(pid)
	assert.Nil(ts.Ts.T, err, "evict")
}

func TestEvict(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	pid := spawnSpinner(mrts.GetRealm(test.REALM1))

	go evict(mrts.GetRealm(test.REALM1), pid)

	status, err := mrts.GetRealm(test.REALM1).WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusEvicted(), "WaitExit status")
}

func TestEvictN(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	N := int(linuxsched.GetNCores())

	pids := []sp.Tpid{}
	for i := 0; i < N; i++ {
		pid := spawnSpinner(mrts.GetRealm(test.REALM1))
		pids = append(pids, pid)
		go evict(mrts.GetRealm(test.REALM1), pid)
	}

	for i := 0; i < N; i++ {
		status, err := mrts.GetRealm(test.REALM1).WaitExit(pids[i])
		assert.Nil(t, err, "WaitExit")
		assert.True(t, status != nil && status.IsStatusEvicted(), "WaitExit status")
	}
}

func TestReserveCores(t *testing.T) {
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "SpawnBurst test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	start := time.Now()
	pid := sp.Tpid("sleeper-aaaaaaa")
	majorityCpu := 1000 * (linuxsched.GetNCores()/2 + 1)
	spawnSleeperMcpu(mrts.GetRealm(test.REALM1), pid, proc.Tmcpu(majorityCpu), SLEEP_MSECS)

	err := mrts.GetRealm(test.REALM1).WaitStart(pid)
	assert.Nil(t, err, "WaitStart error")

	// Make sure pid1 is alphabetically sorted after pid, to ensure that this
	// proc is only picked up *after* the other one.
	pid1 := sp.Tpid("sleeper-bbbbbb")
	spawnSleeperMcpu(mrts.GetRealm(test.REALM1), pid1, proc.Tmcpu(majorityCpu), SLEEP_MSECS)

	status, err := mrts.GetRealm(test.REALM1).WaitExit(pid)
	assert.Nil(t, err, "WaitExit")
	assert.True(t, status != nil && status.IsStatusOK(), "WaitExit status")

	// Make sure the second proc didn't finish
	checkSleeperResult(mrts.GetRealm(test.REALM1), pid)
	checkSleeperResultFalse(mrts.GetRealm(test.REALM1), pid1)

	cleanSleeperResult(mrts.GetRealm(test.REALM1), pid)

	status, err = mrts.GetRealm(test.REALM1).WaitExit(pid1)
	assert.Nil(t, err, "WaitExit 2")
	assert.True(t, status != nil && status.IsStatusOK(), "WaitExit status 2: %v", status)
	end := time.Now()

	assert.True(t, end.Sub(start) > (SLEEP_MSECS*2)*time.Millisecond, "Parallelized")

	checkSleeperResult(mrts.GetRealm(test.REALM1), pid1)

	cleanSleeperResult(mrts.GetRealm(test.REALM1), pid1)
}

func TestWaitExitSimpleMultiKernel1(t *testing.T) {
	waitExitSimpleMultiKernel(t, 1)
}

func TestWaitExitSimpleMultiKernel3(t *testing.T) {
	waitExitSimpleMultiKernel(t, 3)
}

func waitExitSimpleMultiKernel(t *testing.T, n int) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	err := mrts.GetRealm(test.REALM1).BootNode(n)
	assert.Nil(t, err, "Boot node: %v", err)
	db.DPrintf(db.TEST, "Done boot node %d", n)

	sts, err := mrts.GetRealm(test.REALM1).GetDir(sp.MSCHED)
	kernels := sp.Names(sts)
	db.DPrintf(db.TEST, "Kernels %v", kernels)

	p := spawnWaitSleeper(mrts.GetRealm(test.REALM1), []string{kernels[0]})
	assert.Equal(t, kernels[0], p.GetKernelID())

	for i := 1; i < n+1; i++ {
		p := spawnWaitSleeper(mrts.GetRealm(test.REALM1), []string{kernels[i]})
		assert.Equal(t, kernels[i], p.GetKernelID())
	}
}

func TestSpawnBurst(t *testing.T) {
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "SpawnBurst test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	// Number of spinners to burst-spawn
	N := (linuxsched.GetNCores()) * N_NODES

	// Start a couple new procds.
	for i := 0; i < N_NODES; i++ {
		err := mrts.GetRealm(test.REALM1).BootNode(1)
		assert.Nil(t, err, "BootNode %v", err)
	}

	db.DPrintf(db.TEST, "Start burst spawn %v", N)

	ps := burstSpawnSpinner(mrts.GetRealm(test.REALM1), 4)

	for _, p := range ps {
		err := mrts.GetRealm(test.REALM1).WaitStart(p.GetPid())
		assert.Nil(t, err, "WaitStart: %v", err)
	}

	db.DPrintf(db.TEST, "Evict burst spawn")

	for _, p := range ps {
		err := mrts.GetRealm(test.REALM1).Evict(p.GetPid())
		assert.Nil(t, err, "Evict: %v", err)
	}

	db.DPrintf(db.TEST, "Evict wait/exit spawn")

	for _, p := range ps {
		status, err := mrts.GetRealm(test.REALM1).WaitExit(p.GetPid())
		assert.Nil(t, err, "WaitExit: %v", err)
		assert.True(t, status != nil && status.IsStatusEvicted(), "%v: Wrong status: %v", p.GetPid(), status)
	}
}

func TestSpawnManyProcsParallel(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	const N_CONCUR = 5  // 13
	const N_SPAWNS = 50 // 500

	err := mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	done := make(chan int)

	for i := 0; i < N_CONCUR; i++ {
		go func(i int) {
			for j := 0; j < N_SPAWNS; j++ {
				pid := sp.GenPid("sleeper")
				db.DPrintf(db.TEST, "Prep spawn %v", pid)
				a := proc.NewProcPid(pid, "sleeper", []string{"0ms", "name/"})
				err := mrts.GetRealm(test.REALM1).Spawn(a)
				assert.Nil(t, err, "Spawn err %v", err)
				db.DPrintf(db.TEST, "Done spawn %v", pid)

				db.DPrintf(db.TEST, "Prep WaitStart %v", pid)
				err = mrts.GetRealm(test.REALM1).WaitStart(a.GetPid())
				db.DPrintf(db.TEST, "Done WaitStart %v", pid)
				assert.Nil(t, err, "WaitStart error")

				db.DPrintf(db.TEST, "Prep WaitExit %v", pid)
				status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
				db.DPrintf(db.TEST, "Done WaitExit %v", pid)
				assert.Nil(t, err, "WaitExit")
				assert.True(t, status != nil && status.IsStatusOK(), "Status not OK: %v", status)
				cleanSleeperResult(mrts.GetRealm(test.REALM1), pid)
			}
			done <- i
		}(i)
	}
	for i := 0; i < N_CONCUR; i++ {
		x := <-done
		db.DPrintf(db.TEST, "Done %v", x)
	}
}

func TestProcManyOK(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	a := proc.NewProc("proctest", []string{NPROC, BURST, "sleeper", "1us", ""})
	err := mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = mrts.GetRealm(test.REALM1).WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	assert.Nil(t, err, "waitexit")
	assert.True(t, status.IsStatusOK(), status)
	assert.True(t, status.Data().(float64) == 0)
}

func TestProcManyCrash(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	a := proc.NewProc("proctest", []string{NPROC, BURST, "crash"})
	em := crash.NewTeventMapOne(crash.NewEvent(crash.CRASH_CRASH, 0, float64(1.0)))
	err := em.AppendEnv(a)
	assert.Nil(t, err)
	err = mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = mrts.GetRealm(test.REALM1).WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	assert.Nil(t, err, "waitexit")
	assert.True(t, status.IsStatusOK(), status)
	assert.True(t, status.Data().(float64) > 0)
}

func TestProcManyPartition(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	a := proc.NewProc("proctest", []string{NPROC1, BURST, "crash"})
	em := crash.NewTeventMapOne(crash.NewEvent(crash.CRASH_PARTITION, 0, float64(1.0)))
	err := em.AppendEnv(a)
	assert.Nil(t, err, "Spawn")
	err = mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Spawn")
	err = mrts.GetRealm(test.REALM1).WaitStart(a.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	assert.Nil(t, err, "waitexit")
	assert.True(t, status.IsStatusOK(), status)
	assert.True(t, status.Data().(float64) > 0)
}

func TestSpawnCrashLCSched(t *testing.T) {
	const T = 1000
	fn := sp.NAMED + "crashlc.sem"

	e := crash.NewEventPath(crash.LCSCHED_CRASH, T, float64(1.0), fn)
	em := crash.NewTeventMapOne(e)
	err := crash.SetSigmaFail(em)
	assert.Nil(t, err)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	db.DPrintf(db.TEST, "Spawn proc which will queue forever")

	// Spawn a proc which can't possibly be run by any msched.
	pid := spawnSpinnerMcpu(mrts.GetRealm(test.REALM1), proc.Tmcpu(1000*linuxsched.GetNCores()*2))

	db.DPrintf(db.TEST, "Crash an lcsched")

	err = crash.SignalFailer(mrts.GetRoot().FsLib, fn)
	assert.Nil(t, err, "Err signalfailer: %v", err)
	time.Sleep(T * time.Millisecond)

	err = mrts.GetRealm(test.REALM1).WaitStart(pid)
	assert.NotNil(t, err, "WaitStart: %v", err)

	db.DPrintf(db.TEST, "WaitStart done")

	_, err = mrts.GetRealm(test.REALM1).WaitExit(pid)
	assert.NotNil(t, err, "WaitExit: %v", err)

	db.DPrintf(db.TEST, "WaitExit done")
}

func TestCrashMSched(t *testing.T) {
	const T = 1000
	fn0 := sp.NAMED + "crashms0.sem"
	e0 := crash.NewEventPath(crash.MSCHED_CRASH, T, float64(1.0), fn0)
	em := crash.NewTeventMapOne(e0)
	err := crash.SetSigmaFail(em)
	assert.Nil(t, err)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	N_REPL := 3
	OUTDIR := "name/spinner-ephs"

	db.DPrintf(db.TEST, "Boot node 2")
	// Start a couple new nodes.
	fn1 := sp.NAMED + "crashms1.sem"
	e1 := crash.NewEventPath(crash.MSCHED_CRASH, T, float64(1.0), fn1)
	em = crash.NewTeventMapOne(e1)
	err = crash.SetSigmaFail(em)
	assert.Nil(t, err)
	err = mrts.GetRealm(test.REALM1).BootNode(1)

	err = crash.SetSigmaFail(crash.NewTeventMap())
	assert.Nil(t, err, "BootNode %v", err)
	db.DPrintf(db.TEST, "Boot node 3")
	err = mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootNode %v", err)
	db.DPrintf(db.TEST, "Done booting nodes")

	mrts.GetRealm(test.REALM1).RmDir(OUTDIR)
	err = mrts.GetRealm(test.REALM1).MkDir(OUTDIR, 0777)
	assert.Nil(t, err, "Mkdir")

	db.DPrintf(db.TEST, "Rm out dir done")

	// Start a bunch of replicated spinner procs.
	cfg := procgroupmgr.NewProcGroupConfig(N_REPL, "spinner", []string{}, 0, OUTDIR)
	pgm := cfg.StartGrpMgr(mrts.GetRealm(test.REALM1).SigmaClnt)
	db.DPrintf(db.TEST, "GrpMgr started")

	// Wait for them to spawn.
	time.Sleep(5 * time.Second)

	// Make sure they spawned correctly.
	st, err := mrts.GetRealm(test.REALM1).GetDir(OUTDIR)
	if assert.Nil(t, err, "readdir1 err: %v", err) {
		assert.Equal(t, N_REPL, len(st), "wrong num spinners check #1")
	}
	db.DPrintf(db.TEST, "Get OutDir")

	for j, fn := range []string{fn0, fn1} {
		err = crash.SignalFailer(mrts.GetRoot().FsLib, fn)
		assert.Nil(t, err, "crash msched")

		success := false
		for i := 0; i < 10; i++ {
			// Wait for them to respawn.
			time.Sleep(sp.EtcdSessionExpired * time.Second)
			// Check if the spinners are still up
			st, err := mrts.GetRealm(test.REALM1).GetDir(OUTDIR)
			if err != nil {
				db.DPrintf(db.TEST, "Couldn't get OutDir trial %v", i)
				continue
			}
			if len(st) != N_REPL {
				db.DPrintf(db.TEST, "Spinners not up yet: %v != %v", len(st), N_REPL)
				continue
			}
			success = true
			break
		}
		assert.True(t, success, "Spinners never respawned check #%v", j)
		ns := pgm.Nstart()
		n := 0
		for _, m := range ns {
			n += m
		}
		db.DPrintf(db.TEST, "ns %v %d", ns, n)
		assert.True(t, n > (j+1)*N_REPL)
	}

	pgm.StopGroup()
	db.DPrintf(db.TEST, "Stopped GroupMgr")

	// don't check for errors because between seeing the spinner file
	// exists and deleting it, the lease may have expired.
	mrts.GetRealm(test.REALM1).RmDir(OUTDIR)
}
