package sync_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
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
	t *testing.T
	*kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemNamed("sync_test", "..")
	ts.Mkdir(np.LOCKS, 0777)
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
	fsl := fslib.MakeFsLibAddr(fmt.Sprintf("consumer-%v", id), fslib.Named())
	fb := usync.MakeFilePriorityBag(fsl, FILE_BAG_PATH)

	for i := 0; i < nFiles; i++ {
		iStr := fmt.Sprintf("%v", i)
		err := fb.Put("0", iStr, []byte(iStr))
		assert.Nil(ts.t, err, "Error producer put: %v", err)
	}

	done.Done()
}

func testOneWaiterBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 1
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.Shutdown()
}

func testOneWaiterSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 1
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.Shutdown()
}

func testNWaitersOneCondBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.Shutdown()
}

func testNWaitersOneCondSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 1
	runCondWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.Shutdown()
}

// 2021/12/29 19:38:57 Error ReadDir name/cond in Cond.Signal: EOF
func testNWaitersNCondsBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 20
	runCondWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.Shutdown()
}

// XXX  Error ReadDir name/cond in Cond.Signal: EOF
func testNWaitersNCondsSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 20
	runCondWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.Shutdown()
}

// XXX broken name/locks/name-cond-DIR_LOCK, file not found
func testFilePriorityBag(t *testing.T) {
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

	ts.Shutdown()
}

func TestSemaphore(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(WAIT_PATH, 0777)
	assert.Nil(ts.t, err, "Mkdir")
	fsl0 := fslib.MakeFsLibAddr("sem0", fslib.Named())
	fsl1 := fslib.MakeFsLibAddr("semd1", fslib.Named())

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
	ts.Shutdown()
}
