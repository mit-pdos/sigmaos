package fslib

import (
	"log"
	"strconv"
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

func TestCounter(t *testing.T) {
	const N = 10

	ts := makeTstate(t)
	fd, err := ts.CreateFile("name/cnt", 0777|np.DMTMP, np.OWRITE)
	assert.Equal(t, nil, err)
	b := []byte(strconv.Itoa(0))
	_, err = ts.Write(fd, b)
	assert.Equal(t, nil, err)
	err = ts.Close(fd)
	assert.Equal(t, nil, err)

	ch := make(chan int)

	for i := 0; i < N; i++ {
		go func(i int) {
			ntrial := 0
			for {
				ntrial += 1
				fd, err := ts.Open("name/cnt", np.ORDWR|np.OVERSION)
				assert.Equal(t, nil, err)
				b, err := ts.Read(fd, 100)
				if err != nil && err.Error() == "Version mismatch" {
					continue
				}
				assert.Equal(t, nil, err)
				n, err := strconv.Atoi(string(b))
				assert.Equal(t, nil, err)
				n += 1
				b = []byte(strconv.Itoa(n))
				err = ts.Lseek(fd, 0)
				assert.Equal(t, nil, err)
				_, err = ts.Write(fd, b)
				if err != nil && err.Error() == "Version mismatch" {
					continue
				}
				assert.Equal(t, nil, err)
				ts.Close(fd)
				break
			}
			// log.Printf("%d: tries %v\n", i, ntrial)
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
	}
	fd, err = ts.Open("name/cnt", np.ORDWR)
	assert.Equal(t, nil, err)
	b, err = ts.Read(fd, 100)
	assert.Equal(t, nil, err)
	n, err := strconv.Atoi(string(b))
	assert.Equal(t, nil, err)

	assert.Equal(t, N, n)

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
			fsl := MakeFsLib("fslibtest" + strconv.Itoa(i))
			_, err := fsl.CreateFile("name/lock", 0777|np.DMTMP, np.OWRITE|np.OCEXEC)
			assert.Equal(t, nil, err)
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
		// log.Printf("%d acquired lock\n", j)
		err := ts.Remove("name/lock")
		assert.Equal(t, nil, err)
	}
	ts.s.Shutdown(ts.FsLib)
}

func TestConcur(t *testing.T) {
	const N = 10
	ts := makeTstate(t)
	ch := make(chan int)
	for i := 0; i < N; i++ {
		go func(i int) {
			for j := 0; j < 1000; j++ {
				fn := "name/f" + strconv.Itoa(i)
				_, err := ts.CreateFile(fn, 0777|np.DMTMP, np.OWRITE|np.OCEXEC)
				assert.Equal(t, nil, err)
				err = ts.Remove(fn)
				assert.Equal(t, nil, err)
			}
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		j := <-ch
		log.Printf("%d done\n", j)
	}
	ts.s.Shutdown(ts.FsLib)
}
