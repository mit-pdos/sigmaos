package fslib

import (
	// "log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	db "ulambda/debug"
	"ulambda/fsclnt"
)

type Tstate struct {
	*FsLib
	t *testing.T
	s *System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	s, err := BootMin("..")
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.FsLib = MakeFsLib("fslibtest")
	ts.s = s
	ts.t = t
	return ts
}

func TestSymlink(t *testing.T) {
	ts := makeTstate(t)

	var err error
	ts.s.schedd, err = run("..", "/bin/schedd", nil)
	assert.Nil(t, err, "bin/schedd")
	time.Sleep(100 * time.Millisecond)

	db.SetDebug(false)

	b, err := ts.ReadFile(SCHED)
	assert.Nil(t, err, SCHED)
	assert.Equal(t, true, fsclnt.IsRemoteTarget(string(b)))

	sts, err := ts.ReadDir(SCHED + "/")
	assert.Nil(t, err, SCHED+"/")
	assert.Equal(t, 0, len(sts))

	// shutdown schedd
	err = ts.Remove(SCHED + "/")
	assert.Nil(t, err, "Remove")

	time.Sleep(100 * time.Millisecond)

	// start schedd
	ts.s.schedd, err = run("..", "/bin/schedd", nil)
	assert.Nil(t, err, "bin/schedd")
	time.Sleep(100 * time.Millisecond)

	b1, err := ts.ReadFile(SCHED)
	assert.Nil(t, err, SCHED)
	assert.Equal(t, true, fsclnt.IsRemoteTarget(string(b)))
	assert.NotEqual(t, b, b1)

	ts.s.Shutdown(ts.FsLib)
}
