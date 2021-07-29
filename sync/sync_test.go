package sync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
)

const (
	PID       = "test-PID"
	COND_PATH = "name/cond"
	LOCK_DIR  = "name/cond-locks"
	LOCK_NAME = "test-lock"
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

func TestHelloWorld(t *testing.T) {
	ts := makeTstate(t)

	assert.True(ts.t, true, "test")

	ts.s.Shutdown(ts.FsLib)
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

	lock := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME)
	cond := MakeCond(ts.FsLib, PID, COND_PATH, lock)

	done := make(chan int)
	go waiter(ts, cond, done, 0, false)

	// Make sure we don't miss the signal
	time.Sleep(250 * time.Millisecond)

	cond.Signal()

	res := <-done
	assert.Equal(ts.t, 0, res, "Bad done result")

	ts.s.Shutdown(ts.FsLib)
}

func TestOneWaiterBroadcast(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	lock := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME)
	cond := MakeCond(ts.FsLib, PID, COND_PATH, lock)

	done := make(chan int)
	go waiter(ts, cond, done, 0, false)

	// Make sure we don't miss the signal
	time.Sleep(250 * time.Millisecond)

	cond.Broadcast()

	res := <-done
	assert.Equal(ts.t, 0, res, "Bad done result")

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersSignal(t *testing.T) {
	ts := makeTstate(t)

	N := 20
	_ = N

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	lock := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME)
	cond := MakeCond(ts.FsLib, PID, COND_PATH, lock)

	sum := 0

	done := make(chan int)
	for i := 0; i < N; i++ {
		go waiter(ts, cond, done, i, true)
		sum += i
	}

	// Make sure we don't miss the signal
	time.Sleep(250 * time.Millisecond)

	cond.Signal()

	for i := 0; i < N; i++ {
		sum -= <-done
	}
	assert.Equal(ts.t, 0, sum, "Bad sum")

	ts.s.Shutdown(ts.FsLib)
}

func TestNWaitersBroadcast(t *testing.T) {
	ts := makeTstate(t)

	N := 20
	_ = N

	err := ts.Mkdir(LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	lock := MakeLock(ts.FsLib, LOCK_DIR, LOCK_NAME)
	cond := MakeCond(ts.FsLib, PID, COND_PATH, lock)

	sum := 0

	done := make(chan int)
	for i := 0; i < N; i++ {
		go waiter(ts, cond, done, i, false)
		sum += i
	}

	// Make sure we don't miss the signal
	time.Sleep(250 * time.Millisecond)

	cond.Broadcast()

	for i := 0; i < N; i++ {
		sum -= <-done
	}
	assert.Equal(ts.t, 0, sum, "Bad sum")

	ts.s.Shutdown(ts.FsLib)
}
