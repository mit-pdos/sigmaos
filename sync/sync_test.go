package sync

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
)

const (
	PID           = "test-PID"
	COND_PATH     = "name/cond"
	LOCK_DIR      = "name/cond-locks"
	LOCK_NAME     = "test-lock"
	BROADCAST_REL = "broadcast"
	SIGNAL_REL    = "signal"
	FILE_BAG_PATH = "name/filebag"
)

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := kernel.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	db.Name("sync_test")

	ts.FsLib = fslib.MakeFsLib("sync_test")
	ts.t = t
	ts.Mkdir(fslib.LOCKS, 0777)
	return ts
}

func eventWaiter(ts *Tstate, e *Event, done chan int, id int) {
	// Wait, and then possibly signal future waiters
	e.Wait()
	done <- id
}

func runEventWaiters(ts *Tstate, n_waiters, n_events int, destroy bool) {
	events := []*Event{}

	for i := 0; i < n_events; i++ {
		events = append(events, MakeEvent(ts.FsLib, COND_PATH))
	}

	events[0].Init()

	sum := 0

	done := make(chan int)
	for i := 0; i < n_waiters; i++ {
		go eventWaiter(ts, events[i%len(events)], done, i)
		sum += i
	}

	// Make sure we don't miss the signal
	if !destroy {
		time.Sleep(500 * time.Millisecond)
	}

	events[0].Broadcast()

	if destroy {
		events[0].Destroy()
	}

	for i := 0; i < n_waiters; i++ {
		next := <-done
		sum -= next
	}
	assert.Equal(ts.t, 0, sum, "Bad sum")
}

func condWaiter(ts *Tstate, c *Cond, done chan int, id int, signal bool) {
	err := ts.LockFile(LOCK_DIR, LOCK_NAME)
	assert.Nil(ts.t, err, "LockFile waiter [%v]: %v", id, err)

	// Wait, and then possibly signal future waiters
	c.Wait()
	done <- id
	if signal {
		c.Signal()
	}

	err = ts.UnlockFile(LOCK_DIR, LOCK_NAME)
	assert.Nil(ts.t, err, "UnlockFile waiter [%v]: %v", id, err)
}

func runCondWaiters(ts *Tstate, n_waiters, n_conds int, releaseType string) {
	lock := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME, true)
	conds := []*Cond{}

	for i := 0; i < n_conds; i++ {
		conds = append(conds, MakeCond(ts.FsLib, COND_PATH, lock))
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

func fileBagConsumer(ts *Tstate, fb *FileBag, id int, ctr *uint64) {
	for {
		name, contents, err := fb.Get()
		assert.Nil(ts.t, err, "Error consumer get: %v", err)
		assert.Equal(ts.t, name, string(contents), "Error consumer contents and fname not equal")
		atomic.AddUint64(ctr, 1)
	}
}

func fileBagProducer(ts *Tstate, id, nFiles int, done *sync.WaitGroup) {
	fsl := fslib.MakeFsLib(fmt.Sprintf("consumer-%v", id))
	fb := MakeFileBag(fsl, FILE_BAG_PATH)

	for i := 0; i < nFiles; i++ {
		iStr := fmt.Sprintf("%v", i)
		err := fb.Put(iStr, []byte(iStr))
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

	lock := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME, true)

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

	ts.s.Shutdown(ts.FsLib)
}

func TestLock2(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	N := 20

	lock1 := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME+"-1234", true)
	lock2 := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME+"-1234", true)

	for i := 0; i < N; i++ {
		lock1.Lock()
		lock1.Unlock()
		lock2.Lock()
		lock2.Unlock()
	}

	ts.s.Shutdown(ts.FsLib)
}

func TestLock3(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	N := 3000
	n_threads := 20
	cnt := 0

	lock := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME+"-1234", true)

	var done sync.WaitGroup
	done.Add(n_threads)

	for i := 0; i < n_threads; i++ {
		go func(done *sync.WaitGroup, lock *Lock, N *int, cnt *int) {
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

	ts.s.Shutdown(ts.FsLib)
}

func TestLock4(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	fsl1 := fslib.MakeFsLib("fslib-1")
	fsl2 := fslib.MakeFsLib("fslib-1")

	lock1 := MakeLock(fsl1, LOCK_DIR, LOCK_NAME, true)
	//	lock2 := MakeLock(fsl2, LOCK_DIR, LOCK_NAME, true)

	// Establish a connection
	_, err = fsl2.ReadDir(LOCK_DIR)

	assert.Nil(ts.t, err, "ReadDir")

	lock1.Lock()

	fsl2.Exit()

	time.Sleep(2 * time.Second)

	lock1.Unlock()

	ts.s.Shutdown(ts.FsLib)
}

func TestOneWaiterBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 1
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestOneWaiterSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 1
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersOneCondBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersOneCondSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersNCondsBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 20
	runCondWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersNCondsSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 20
	runCondWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestOneWaiterOneEvent(t *testing.T) {
	ts := makeTstate(t)

	n_waiters := 1
	n_events := 1
	runEventWaiters(ts, n_waiters, n_events, false)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersOneEvent(t *testing.T) {
	ts := makeTstate(t)

	n_waiters := 20
	n_events := 1
	runEventWaiters(ts, n_waiters, n_events, false)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersNEvents(t *testing.T) {
	ts := makeTstate(t)

	n_waiters := 20
	n_events := 20
	runEventWaiters(ts, n_waiters, n_events, false)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersNEventsDestroy(t *testing.T) {
	ts := makeTstate(t)

	n_waiters := 20
	n_events := 20
	runEventWaiters(ts, n_waiters, n_events, true)

	ts.s.Shutdown(ts.FsLib)
}

func TestFileBag(t *testing.T) {
	ts := makeTstate(t)

	n_consumers := 39
	n_producers := 1
	n_files := 500
	n_files_per_producer := n_files / n_producers

	var done sync.WaitGroup
	done.Add(n_producers)

	//	fsl := fslib.MakeFsLib(fmt.Sprintf("consumer-%v", i))
	fb := MakeFileBag(ts.FsLib, FILE_BAG_PATH)

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

	ts.s.Shutdown(ts.FsLib)
}
