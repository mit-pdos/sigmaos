package sync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
)

const (
	PID           = "test-PID"
	COND_PATH     = "name/cond"
	LOCK_DIR      = "name/cond-locks"
	LOCK_NAME     = "test-lock"
	BROADCAST_REL = "broadcast"
	SIGNAL_REL    = "signal"
)

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *fslib.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s
	db.Name("sync_test")

	ts.FsLib = fslib.MakeFsLib("sync_test")
	ts.t = t
	return ts
}

func waiter(ts *Tstate, c *Cond, done chan int, id int, signal bool) {
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

func runWaiters(ts *Tstate, n_waiters, n_conds int, releaseType string) {
	lock := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME)
	conds := []*Cond{}

	for i := 0; i < n_conds; i++ {
		conds = append(conds, MakeCond(ts.FsLib, PID, COND_PATH, lock))
	}

	conds[0].Init()

	sum := 0

	done := make(chan int)
	for i := 0; i < n_waiters; i++ {
		go waiter(ts, conds[i%len(conds)], done, i, n_waiters > 1 && releaseType == SIGNAL_REL)
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

func TestLock(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	N := 20

	sum := 0
	current := 0
	done := make(chan int)

	lock := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME)

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

func TestOneWaiterSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 1
	n_conds := 1
	runWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestOneWaiterBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 1
	n_conds := 1
	runWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersOneCondSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 1
	runWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersOneCondBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 1
	runWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersNCondsSignal(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 20
	runWaiters(ts, n_waiters, n_conds, SIGNAL_REL)

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersNCondsBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	n_waiters := 20
	n_conds := 20
	runWaiters(ts, n_waiters, n_conds, BROADCAST_REL)

	ts.s.Shutdown(ts.FsLib)
}
