package fslib

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fsclnt"
	np "ulambda/ninep"
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
	db.Name("fslib_test")
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

func TestVersion(t *testing.T) {
	ts := makeTstate(t)

	fd, err := ts.CreateFile("name/xxx", np.OWRITE)
	assert.Nil(t, err, "CreateFile")
	buf := make([]byte, 1000)
	off, err := ts.WriteV(fd, buf)
	assert.Nil(t, err, "Vwrite0")
	assert.Equal(t, np.Tsize(1000), off)
	err = ts.Remove("name/xxx")
	assert.Nil(t, err, "Remove")
	off, err = ts.WriteV(fd, buf)
	assert.Equal(t, err.Error(), "Version mismatch")
	_, err = ts.ReadV(fd, np.Tsize(1000))
	assert.Equal(t, err.Error(), "Version mismatch")

	ts.s.Shutdown(ts.FsLib)
}
