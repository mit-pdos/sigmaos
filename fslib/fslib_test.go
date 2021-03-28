package fslib

import (
	"log"
	"strings"
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

	fd, err := ts.CreateFile("name/xxx", 0777, np.OWRITE|np.OVERSION)
	assert.Nil(t, err, "CreateFile")
	buf := make([]byte, 1000)
	off, err := ts.Write(fd, buf)
	assert.Nil(t, err, "Vwrite0")
	assert.Equal(t, np.Tsize(1000), off)
	err = ts.Remove("name/xxx")
	assert.Nil(t, err, "Remove")
	off, err = ts.Write(fd, buf)
	assert.Equal(t, err.Error(), "Version mismatch")
	_, err = ts.Read(fd, np.Tsize(1000))
	assert.Equal(t, err.Error(), "Version mismatch")

	ts.s.Shutdown(ts.FsLib)
}

func TestEphemeral(t *testing.T) {
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

	time.Sleep(100 * time.Millisecond)

	ts.s.Kill(SCHED)

	time.Sleep(100 * time.Millisecond)

	_, err = ts.ReadFile(SCHED)
	assert.NotEqual(t, nil, err)
	if err != nil {
		assert.Equal(t, true, strings.HasPrefix(err.Error(), "file not found"))
	}

	log.Printf("Shutdown\n")

	ts.s.Shutdown(ts.FsLib)
}

func TestLock(t *testing.T) {
	const N = 10

	ts := makeTstate(t)
	ch := make(chan int)
	for i := 0; i < N; i++ {
		go func(i int) {
			_, err := ts.CreateFile("name/lock", 0777|np.DMTMP, np.OWRITE|np.OCEXEC)
			assert.Equal(t, nil, err)
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		j := <-ch
		log.Printf("%d acquired lock\n", j)
		err := ts.Remove("name/lock")
		assert.Equal(t, nil, err)
	}
	ts.s.Shutdown(ts.FsLib)
}
