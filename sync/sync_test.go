package sync_test

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/realm"
	usync "ulambda/sync"
)

const (
	PID           = "test-PID"
	COND_PATH     = "name/cond"
	LOCK_DIR      = "name/cond-locks"
	LOCK_NAME     = "test-lock"
	BROADCAST_REL = "broadcast"
	SIGNAL_REL    = "signal"
	FILE_BAG_PATH = "name/filebag"
	WAIT_PATH     = "name/wait"
)

type Tstate struct {
	*procclnt.ProcClnt
	*fslib.FsLib
	t   *testing.T
	e   *realm.TestEnv
	cfg *realm.RealmConfig
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg
	db.Name("sync_test")
	ts.FsLib = fslib.MakeFsLibAddr("sync_test", ts.cfg.NamedAddr)

	ts.ProcClnt = procclnt.MakeProcClntInit(ts.FsLib, cfg.NamedAddr)

	ts.t = t
	ts.Mkdir(named.LOCKS, 0777)
	return ts
}

func condWaiter(ts *Tstate, c *usync.Cond, done chan int, id int, signal bool) {
	l := usync.MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME, true)
	l.Lock()

	// Wait, and then possibly signal future waiters
	c.Wait()
	done <- id
	if signal {
		c.Signal()
	}

	l.Unlock()
}

func runCondWaiters(ts *Tstate, n_waiters, n_conds int, releaseType string) {
	lock := usync.MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME, true)
	conds := []*usync.Cond{}

	for i := 0; i < n_conds; i++ {
		conds = append(conds, usync.MakeCond(ts.FsLib, COND_PATH, lock, true))
	}

	conds[0].Init()

	sum := 0

	done := make(chan int)
	for i := 0; i < n_waiters; i++ {
		go condWaiter(ts, conds[i%len(conds)], done, i, n_waiters > 1 && releaseType == SIGNAL_REL)
		sum += i
	}

	// Make sure we don't miss the signal
	time.Sleep(500 * time.Millisecond)

	switch releaseType {
	case BROADCAST_REL:
		conds[0].Broadcast()
	case SIGNAL_REL:
		conds[0].Signal()
	default:
		assert.True(ts.t, false, "Invalid release type")
	}

	for i := 0; i < n_waiters; i++ {
		sum -= <-done
	}
	assert.Equal(ts.t, 0, sum, "Bad sum")
}

func fileBagConsumer(ts *Tstate, fb *usync.FilePriorityBag, id int, ctr *uint64) {
	for {
		_, name, contents, err := fb.Get()
		if err != nil && err.Error() == "EOF" {
			// terminate on end of file
			return
		}
		assert.Nil(ts.t, err, "Error consumer get: %v", err)
		assert.Equal(ts.t, name, string(contents), "Error consumer contents and fname not equal")
		atomic.AddUint64(ctr, 1)
	}
}

func fileBagProducer(ts *Tstate, id, nFiles int, done *sync.WaitGroup) {
	fsl := fslib.MakeFsLibAddr(fmt.Sprintf("consumer-%v", id), ts.cfg.NamedAddr)
	fb := usync.MakeFilePriorityBag(fsl, FILE_BAG_PATH)

	for i := 0; i < nFiles; i++ {
		iStr := fmt.Sprintf("%v", i)
		err := fb.Put("0", iStr, []byte(iStr))
		assert.Nil(ts.t, err, "Error producer put: %v", err)
	}

	done.Done()
}

func TestLock1(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	N := 100

	sum := 0
	current := 0
	done := make(chan int)

	lock := usync.MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME, true)

	for i := 0; i < N; i++ {
		go func(i int) {
			me := false
			for !me {
				lock.Lock()
				if current == i {
					current += 1
					done <- i
					me = true
				}
				lock.Unlock()
			}
		}(i)
		sum += i
	}

	for i := 0; i < N; i++ {
		next := <-done
		assert.Equal(ts.t, i, next, "Next (%v) not equal to expected (%v)", next, i)
	}

	ts.e.Shutdown()
}

func TestLock2(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	N := 20

	lock1 := usync.MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME+"-1234", true)
	lock2 := usync.MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME+"-1234", true)

	for i := 0; i < N; i++ {
		lock1.Lock()
		lock1.Unlock()
		lock2.Lock()
		lock2.Unlock()
	}

	ts.e.Shutdown()
}

func TestLock3(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	N := 3000
	n_threads := 20
	cnt := 0

	lock := usync.MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME+"-1234", true)

	var done sync.WaitGroup
	done.Add(n_threads)

	for i := 0; i < n_threads; i++ {
		go func(done *sync.WaitGroup, lock *usync.Lock, N *int, cnt *int) {
			defer done.Done()
			for {
				lock.Lock()
				if *cnt < *N {
					*cnt += 1
				} else {
					lock.Unlock()
					break
				}
				lock.Unlock()
			}
		}(&done, lock, &N, &cnt)
	}

	done.Wait()
	assert.Equal(ts.t, N, cnt, "Count doesn't match up")

	ts.e.Shutdown()
}

func TestLock4(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	fsl1 := fslib.MakeFsLibAddr("fslib-1", ts.cfg.NamedAddr)
	fsl2 := fslib.MakeFsLibAddr("fslib-1", ts.cfg.NamedAddr)

	lock1 := usync.MakeLock(fsl1, LOCK_DIR, LOCK_NAME, true)
	//	lock2 := MakeLock(fsl2, LOCK_DIR, LOCK_NAME, true)

	// Establish a connection
	_, err = fsl2.ReadDir(LOCK_DIR)

	assert.Nil(ts.t, err, "ReadDir")

	lock1.Lock()

	fsl2.Exit()

	time.Sleep(2 * time.Second)

	lock1.Unlock()

	ts.e.Shutdown()
}

func TestOneWaiterBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 1
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.e.Shutdown()
}

func TestOneWaiterSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 1
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.e.Shutdown()
}

func TestNWaitersOneCondBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.e.Shutdown()
}

func TestNWaitersOneCondSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.e.Shutdown()
}

func TestNWaitersNCondsBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 20
	runCondWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.e.Shutdown()
}

func TestNWaitersNCondsSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 20
	runCondWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.e.Shutdown()
}

func TestFilePriorityBag(t *testing.T) {
	ts := makeTstate(t)

	n_consumers := 39
	n_producers := 1
	n_files := 500
	n_files_per_producer := n_files / n_producers

	var done sync.WaitGroup
	done.Add(n_producers)

	fb := usync.MakeFilePriorityBag(ts.FsLib, FILE_BAG_PATH)

	var ctr uint64 = 0
	for i := 0; i < n_consumers; i++ {
		go fileBagConsumer(ts, fb, i, &ctr)
	}

	for i := 0; i < n_producers; i++ {
		go fileBagProducer(ts, i, n_files_per_producer, &done)
	}

	done.Wait()

	// XXX Wait for convergence in a more principled way...
	time.Sleep(2 * time.Second)

	assert.Equal(ts.t, int(ctr), n_files, "File count is off")

	ts.e.Shutdown()
}

func TestSemaphore(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(WAIT_PATH, 0777)
	assert.Nil(ts.t, err, "Mkdir")
	fsl0 := fslib.MakeFsLibAddr("sem0", ts.cfg.NamedAddr)
	fsl1 := fslib.MakeFsLibAddr("semd1", ts.cfg.NamedAddr)

	for i := 0; i < 100; i++ {
		sem := usync.MakeSemaphore(ts.FsLib, WAIT_PATH+"/x")
		sem.Init()

		ch := make(chan bool)

		go func(ch chan bool) {
			sem := usync.MakeSemaphore(fsl0, WAIT_PATH+"/x")
			sem.Down()
			ch <- true
		}(ch)
		go func(ch chan bool) {
			sem := usync.MakeSemaphore(fsl1, WAIT_PATH+"/x")
			sem.Up()
			ch <- true
		}(ch)

		for i := 0; i < 2; i++ {
			<-ch
		}
	}
	ts.e.Shutdown()
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
	ts.e.Shutdown()
}

func TestLockerNoPart(t *testing.T) {
	testLocker(t, "NO")
}

//func TestLockerWithPart(t *testing.T) {
//	testLocker(t, "YES")
//}
