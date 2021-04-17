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

func TestRename(t *testing.T) {
	ts := makeTstate(t)
	err := ts.Mkdir("name/d1", 0777)
	assert.Equal(t, nil, err)
	err = ts.Mkdir("name/d2", 0777)
	assert.Equal(t, nil, err)

	fn := "name/d1/f"
	fn1 := "name/d2/g"
	d := []byte("hello")
	err = ts.MakeFile(fn, d)
	assert.Equal(t, nil, err)

	err = ts.Rename(fn, fn1)
	assert.Equal(t, nil, err)

	d1, err := ts.ReadFile(fn1)
	assert.Equal(t, "hello", string(d1))
	ts.s.Shutdown(ts.FsLib)
}

func (ts *Tstate) localdName(t *testing.T) string {
	sts, err := ts.ReadDir(LOCALD_ROOT)
	assert.Nil(t, err, LOCALD_ROOT)
	assert.Equal(t, 1, len(sts))
	name := LOCALD_ROOT + "/" + sts[0].Name
	return name
}

func TestSymlink(t *testing.T) {
	ts := makeTstate(t)

	var err error
	ts.s.locald, err = run("..", "/bin/locald", []string{"./"})
	assert.Nil(t, err, "bin/locald")
	time.Sleep(100 * time.Millisecond)

	name := ts.localdName(t)
	b, err := ts.ReadFile(name)
	assert.Nil(t, err, name)
	assert.Equal(t, true, fsclnt.IsRemoteTarget(string(b)))

	sts, err := ts.ReadDir(name + "/")
	assert.Nil(t, err, name+"/")
	assert.Equal(t, 0, len(sts))

	// shutdown locald
	err = ts.Remove(name + "/")
	assert.Nil(t, err, "Remove")

	time.Sleep(100 * time.Millisecond)

	// start schedd
	ts.s.locald, err = run("..", "/bin/locald", []string{"./"})
	assert.Nil(t, err, "bin/locald")
	time.Sleep(100 * time.Millisecond)

	name = ts.localdName(t)

	b1, err := ts.ReadFile(name)
	assert.Nil(t, err, name)
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
	ts.s.locald, err = run("..", "/bin/locald", []string{"./"})
	assert.Nil(t, err, "bin/locald")
	time.Sleep(100 * time.Millisecond)

	name := ts.localdName(t)
	b, err := ts.ReadFile(name)
	assert.Nil(t, err, name)
	assert.Equal(t, true, fsclnt.IsRemoteTarget(string(b)))

	sts, err := ts.ReadDir(name + "/")
	assert.Nil(t, err, name+"/")
	assert.Equal(t, 0, len(sts))

	time.Sleep(100 * time.Millisecond)

	ts.s.Kill(LOCALD)

	time.Sleep(100 * time.Millisecond)

	_, err = ts.ReadFile(name)
	assert.NotEqual(t, nil, err)
	if err != nil {
		assert.Equal(t, true, strings.HasPrefix(err.Error(), "file not found"))
	}

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

func TestWatchRemove(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/w"
	err := ts.MakeFile(fn, nil)
	assert.Equal(t, nil, err)

	ch := make(chan bool)
	err = ts.RemoveWatch(fn, func(string) {
		ch <- true
	})

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	<-ch

	ts.s.Shutdown(ts.FsLib)
}

func TestWatchCreate(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/w"
	ch := make(chan bool)
	_, err := ts.ReadFileWatch(fn, func(string) {
		ch <- true
	})
	assert.NotEqual(t, nil, err)
	if err != nil {
		assert.Equal(t, true, strings.HasPrefix(err.Error(), "file not found"))
	}

	err = ts.MakeFile(fn, nil)
	assert.Equal(t, nil, err)

	<-ch

	ts.s.Shutdown(ts.FsLib)
}

func TestConcur(t *testing.T) {
	const N = 20
	ts := makeTstate(t)
	ch := make(chan int)
	for i := 0; i < N; i++ {
		go func(i int) {
			for j := 0; j < 1000; j++ {
				fn := "name/f" + strconv.Itoa(i)
				data := []byte(fn)
				err := ts.MakeFile(fn, data)
				assert.Equal(t, nil, err)
				d, err := ts.ReadFile(fn)
				assert.Equal(t, nil, err)
				assert.Equal(t, len(data), len(d))
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
