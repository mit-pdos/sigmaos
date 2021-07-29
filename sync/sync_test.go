package sync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
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

func TestHelloWorld(t *testing.T) {
	ts := makeTstate(t)

	Init(ts.FsLib)

	assert.True(ts.t, true, "test")

	ts.s.Shutdown(ts.FsLib)
}

func TestOneWaiterSignal(t *testing.T) {
	ts := makeTstate(t)

	Init(ts.FsLib)

	pid := "test-pid"
	condPath := "name/cond"
	lockDir := "name/cond-locks"
	lockName := "test-lock"

	err := ts.Mkdir(lockDir, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	c := MakeCond(pid, condPath, lockDir, lockName)

	fsl.LockFile(lockDir, lockName)

	done := make(chan bool)
	go func() {
		c.Wait()
		done <- true
	}()

	// Make sure we don't miss the signal
	time.Sleep(250 * time.Millisecond)

	c.Signal()

	res := <-done
	assert.True(ts.t, res, "Bad done result")

	ts.s.Shutdown(ts.FsLib)
}

func TestOneWaiterBroadcast(t *testing.T) {
	ts := makeTstate(t)

	Init(ts.FsLib)

	pid := "test-pid"
	condPath := "name/cond"
	lockDir := "name/cond-locks"
	lockName := "test-lock"

	err := ts.Mkdir(lockDir, 0777)
	assert.Nil(ts.t, err, "Mkdir name/locks: %v", err)

	c := MakeCond(pid, condPath, lockDir, lockName)

	fsl.LockFile(lockDir, lockName)

	done := make(chan bool)
	go func() {
		c.Wait()
		done <- true
	}()

	// Make sure we don't miss the signal
	time.Sleep(250 * time.Millisecond)

	c.Broadcast()

	res := <-done
	assert.True(ts.t, res, "Bad done result")

	ts.s.Shutdown(ts.FsLib)
}
