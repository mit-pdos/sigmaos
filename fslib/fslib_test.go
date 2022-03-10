package fslib_test

import (
	"flag"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/stats"
	"ulambda/test"
)

var path string

func init() {
	flag.StringVar(&path, "path", np.NAMED, "path for file system")
}

func TestInitFs(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	sts, err := ts.GetDir(path)
	assert.Equal(t, nil, err)
	if path == np.NAMED {
		assert.True(t, fslib.Present(sts, named.InitDir), "initfs")
	} else {
		assert.True(t, len(sts) == 0, "initfs")
	}
	ts.Shutdown()
}

func TestRemoveSimple(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := path + "f"
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	_, err = ts.Stat(fn)
	assert.NotEqual(t, nil, err)

	ts.Shutdown()
}

func TestConnect(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := path + "f"
	d := []byte("hello")
	fd, err := ts.Create(fn, 0777, np.OWRITE)
	assert.Equal(t, nil, err)
	_, err = ts.Write(fd, d)
	assert.Equal(t, nil, err)

	ts.Disconnect(path)
	time.Sleep(100 * time.Millisecond)
	log.Printf("disconnected\n")

	_, err = ts.Write(fd, d)
	assert.True(t, np.IsErrUnreachable(err))

	err = ts.Close(fd)
	assert.True(t, np.IsErrUnreachable(err))

	fd, err = ts.Open(fn, np.OREAD)
	assert.True(t, np.IsErrUnreachable(err))

	ts.Shutdown()
}

func TestRemoveNonExistent(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := path + "f"
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove("name/this-file-does-not-exist")
	assert.NotNil(t, err)

	ts.Shutdown()
}

func TestRemovePath(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	d1 := path + "/d1/"
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	fn := d1 + "/f"
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	b, err := ts.GetFile(fn)
	assert.Equal(t, "hello", string(b))

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func TestRename(t *testing.T) {
	d1 := path + "/d1/"
	d2 := path + "/d2/"
	ts := test.MakeTstatePath(t, path)
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir(d2, 0777)
	assert.Equal(t, nil, err)

	fn := d1 + "f"
	fn1 := d2 + "g"
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Rename(fn, fn1)
	assert.Equal(t, nil, err)

	b, err := ts.GetFile(fn1)
	assert.Equal(t, "hello", string(b))
	ts.Shutdown()
}

func TestRenameAndRemove(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	err := ts.MkDir("name/d1", 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir("name/d2", 0777)
	assert.Equal(t, nil, err)

	fn := "name/d1/f"
	fn1 := "name/d2/g"
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Rename(fn, fn1)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(fn1)
	assert.Equal(t, nil, err)
	assert.Equal(t, "hello", string(d1))

	_, err = ts.Stat(fn1)
	assert.Equal(t, nil, err)

	err = ts.Remove(fn1)
	assert.Equal(t, nil, err)
	ts.Shutdown()
}

func TestNonEmpty(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	err := ts.MkDir("name/d1", 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir("name/d2", 0777)
	assert.Equal(t, nil, err)

	fn := "name/d1/f"
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove("name/d1")
	assert.NotNil(t, err, "Remove")

	err = ts.Rename("name/d2", "name/d1")
	assert.NotNil(t, err, "Rename")

	ts.Shutdown()
}

func TestSetAppend(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	d := []byte("1234")
	fn := "name/f"

	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)
	l, err := ts.SetFile(fn, d, np.Toffset(len(d)))
	assert.Equal(t, nil, err)
	assert.Equal(t, np.Tsize(len(d)), l)
	b, err := ts.GetFile(fn)
	assert.Equal(t, nil, err)
	assert.Equal(t, len(d)*2, len(b))
	ts.Shutdown()
}

func TestCopy(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	d := []byte("hello")
	src := "name/f"
	dst := "name/g"
	_, err := ts.PutFile(src, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.CopyFile(src, dst)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(dst)
	assert.Equal(t, "hello", string(d1))

	ts.Shutdown()
}

func TestDirSimple(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	dn := "name/d"
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	b, err := ts.IsDir(dn)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, b)

	d := []byte("hello")
	_, err = ts.PutFile(dn+"/f", 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	sts, err := ts.GetDir(dn)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(sts))
	assert.Equal(t, "f", sts[0].Name)

	err = ts.RmDir(dn)
	_, err = ts.Stat(dn)
	assert.NotEqual(t, nil, err)

	ts.Shutdown()
}

func TestDirDot(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	dn := "name/dir0"
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	b, err := ts.IsDir(dn + "/.")
	assert.Equal(t, nil, err)
	assert.Equal(t, true, b)
	err = ts.RmDir(dn + "/.")
	assert.NotEqual(t, nil, err)
	err = ts.RmDir(dn)
	_, err = ts.Stat(dn + "/.")
	assert.NotEqual(t, nil, err)
	_, err = ts.Stat("name/.")
	assert.Equal(t, nil, err)
	ts.Shutdown()
}

func TestPageDir(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	dn := "name/dir/"
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	ts.SetChunkSz(np.Tsize(512))
	n := 100
	names := make([]string, 0)
	for i := 0; i < n; i++ {
		name := strconv.Itoa(i)
		names = append(names, name)
		_, err := ts.PutFile(dn+name, 0777, np.OWRITE, []byte(name))
		assert.Equal(t, nil, err)
	}
	sort.SliceStable(names, func(i, j int) bool {
		return names[i] < names[j]
	})
	i := 0
	ts.ProcessDir(dn, func(st *np.Stat) (bool, error) {
		assert.Equal(t, names[i], st.Name)
		i += 1
		return false, nil

	})
	assert.Equal(t, i, n)
	ts.Shutdown()
}

// XXX no versions for now
func testCounter(t *testing.T) {
	const N = 10

	ts := test.MakeTstatePath(t, path)

	b := []byte(strconv.Itoa(0))
	_, err := ts.PutFile("name/cnt", 0777|np.DMTMP, np.OWRITE, b)
	assert.Equal(t, nil, err)

	ch := make(chan int)

	for i := 0; i < N; i++ {
		go func(i int) {
			ntrial := 0
			for {
				ntrial += 1
				fd, err := ts.Open("name/cnt", np.ORDWR)
				assert.Equal(t, nil, err)
				b, err := ts.Read(fd, 1000)
				assert.Equal(t, nil, err)
				n, err := strconv.Atoi(string(b))
				assert.Equal(t, nil, err)
				n += 1
				b = []byte(strconv.Itoa(n))
				_, err = ts.Write(fd, b)
				if err != nil && err.Error() == "Version mismatch" {
					continue
				}
				assert.Equal(t, nil, err)
				err = ts.Close(fd)
				assert.Equal(t, nil, err)
				break
			}
			// log.Printf("%d: tries %v\n", i, ntrial)
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
	}
	b, err = ts.GetFile("name/cnt")
	assert.Equal(t, nil, err)
	n, err := strconv.Atoi(string(b))
	assert.Equal(t, nil, err)

	assert.Equal(t, N, n)

	ts.Shutdown()
}

// Inline Set() so that we can delay the Write() to emulate a delay on
// the server between open and write.
func writeFile(fl *fslib.FsLib, fn string, d []byte) error {
	fd, err := fl.Open(fn, np.OWRITE)
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Millisecond)
	_, err = fl.Write(fd, d)
	if err != nil {
		return err
	}
	err = fl.Close(fd)
	if err != nil {
		return err
	}
	return nil
}

func TestWatchCreate(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := "name/w"
	ch := make(chan bool)
	fd, err := ts.OpenWatch(fn, np.OREAD, func(string, error) {
		ch <- true
	})
	assert.NotEqual(t, nil, err)
	assert.Equal(t, -1, fd, err)
	assert.True(t, np.IsErrNotfound(err))

	// give Watch goroutine to start
	time.Sleep(100 * time.Millisecond)

	_, err = ts.PutFile(fn, 0777, np.OWRITE, nil)
	assert.Equal(t, nil, err)

	<-ch

	ts.Shutdown()
}

func TestWatchRemoveOne(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := "name/w"
	_, err := ts.PutFile(fn, 0777, np.OWRITE, nil)
	assert.Equal(t, nil, err)

	ch := make(chan bool)
	err = ts.SetRemoveWatch(fn, func(path string, err error) {
		assert.Equal(t, nil, err, path)
		ch <- true
	})
	assert.Equal(t, nil, err)

	// give Watch goroutine to start
	time.Sleep(100 * time.Millisecond)

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	<-ch

	ts.Shutdown()
}

func TestWatchDir(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := "name/d1"
	err := ts.MkDir(fn, 0777)
	assert.Equal(t, nil, err)

	ch := make(chan bool)
	err = ts.SetDirWatch(fn, func(path string, err error) {
		assert.Equal(t, nil, err, path)
		ch <- true
	})
	assert.Equal(t, nil, err)

	// give Watch goroutine to start
	time.Sleep(100 * time.Millisecond)

	_, err = ts.PutFile(fn+"/x", 0777, np.OWRITE, nil)
	assert.Equal(t, nil, err)

	<-ch

	ts.Shutdown()
}

func TestLock1(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	ch := make(chan int)
	ts.MkDir("name/locks", 0777)

	// Lock the file
	_, err := ts.PutFile("name/locks/test-lock", 0777|np.DMTMP, np.OWRITE|np.OCEXEC, []byte{})
	assert.Equal(t, nil, err)
	fsl := fslib.MakeFsLibAddr("fslibtest0", fslib.Named())
	go func() {
		_, err := fsl.PutFile("name/locks/test-lock", 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
		assert.Nil(t, err, "MakeFile")
		ch <- 0
	}()
	time.Sleep(time.Second * 2)
	err = ts.Remove("name/locks/test-lock")
	assert.Nil(t, err, "Remove")
	go func() {
		time.Sleep(2 * time.Second)
		ch <- 1
	}()
	i := <-ch
	assert.Equal(t, 0, i)

	ts.Shutdown()
}

func TestLockN(t *testing.T) {
	const N = 20

	ts := test.MakeTstatePath(t, path)
	ch := make(chan int)
	acquired := false
	for i := 0; i < N; i++ {
		go func(i int) {
			fsl := fslib.MakeFsLibAddr("fslibtest"+strconv.Itoa(i), fslib.Named())
			_, err := fsl.PutFile("name/lock", 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
			assert.Equal(t, nil, err)
			assert.Equal(t, false, acquired)
			acquired = true
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
		//		log.Printf("%d acquired lock\n", i)
		acquired = false
		err := ts.Remove("name/lock")
		assert.Equal(t, nil, err)
	}
	ts.Shutdown()
}

func TestLockAfterConnClose(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	lPath := "name/lock-conn-close-test"

	fsl1 := fslib.MakeFsLibAddr("fslibtest-1", fslib.Named())

	_, err := ts.PutFile(lPath, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	assert.Nil(t, err, "Make lock 1")

	go func() {
		// Should wait
		_, err := fsl1.PutFile(lPath, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
		assert.NotNil(t, err, "Make lock 2")
	}()

	time.Sleep(500 * time.Millisecond)

	// Kill fsl1's connection
	fsl1.Disconnect(lPath)

	// Remove the lock file
	ts.Remove(lPath)
	assert.Equal(t, nil, err)

	// Try to lock again (should succeed)
	_, err = ts.PutFile(lPath, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	assert.Nil(t, err, "Make lock 3")

	ts.Shutdown()
}

func TestWatchRemoveConcur(t *testing.T) {
	const N = 5_000

	ts := test.MakeTstatePath(t, path)
	dn := "name/d1"
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)

	fn := dn + "/w"

	ch := make(chan error)
	done := make(chan bool)
	go func() {
		fsl := fslib.MakeFsLibAddr("fsl1", fslib.Named())
		for i := 1; i < N; {
			_, err := fsl.PutFile(fn, 0777, np.OWRITE, nil)
			assert.Equal(t, nil, err)
			err = ts.SetRemoveWatch(fn, func(fn string, r error) {
				// log.Printf("watch cb %v err %v\n", i, r)
				ch <- r
			})
			if err == nil {
				r := <-ch
				if r == nil {
					i += 1
				}
			} else {
				// log.Printf("SetRemoveWatch %v err %v\n", i, err)
			}
		}
		done <- true
	}()

	stop := false
	for !stop {
		select {
		case <-done:
			stop = true
		default:
			time.Sleep(1 * time.Millisecond)
			ts.Remove(fn) // remove may fail
		}
	}

	ts.Shutdown()
}

func TestConcurFile(t *testing.T) {
	const N = 20
	ts := test.MakeTstatePath(t, path)
	ch := make(chan int)
	for i := 0; i < N; i++ {
		go func(i int) {
			for j := 0; j < 1000; j++ {
				fn := "name/f" + strconv.Itoa(i)
				data := []byte(fn)
				_, err := ts.PutFile(fn, 0777, np.OWRITE, data)
				assert.Equal(t, nil, err)
				d, err := ts.GetFile(fn)
				assert.Equal(t, nil, err)
				assert.Equal(t, len(data), len(d))
				err = ts.Remove(fn)
				assert.Equal(t, nil, err)
			}
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
	}
	ts.Shutdown()
}

const (
	TODO  = "name/todo"
	DONE  = "name/done"
	NFILE = 1000
)

func initfs(ts *test.Tstate) {
	err := ts.MkDir(TODO, 07000)
	assert.Nil(ts.T, err, "Create done")
	err = ts.MkDir(DONE, 07000)
	assert.Nil(ts.T, err, "Create todo")
}

// Keep renaming files in the todo directory until we failed to rename
// any file
func testRename(ts *test.Tstate, fsl *fslib.FsLib, t string) int {
	ok := true
	i := 0
	for ok {
		ok = false
		sts, err := fsl.GetDir(TODO)
		assert.Nil(ts.T, err, "GetDir")
		for _, st := range sts {
			err = fsl.Rename(TODO+"/"+st.Name, DONE+"/"+st.Name+"."+t)
			if err == nil {
				i = i + 1
				ok = true
			} else {
				assert.True(ts.T, np.IsErrNotfound(err))
			}
		}
	}
	return i
}

func checkFs(ts *test.Tstate) {
	sts, err := ts.GetDir(DONE)
	assert.Nil(ts.T, err, "GetDir")
	assert.Equal(ts.T, NFILE, len(sts), "checkFs")
	files := make(map[int]bool)
	for _, st := range sts {
		n := strings.TrimSuffix(st.Name, filepath.Ext(st.Name))
		n = strings.TrimPrefix(n, "job")
		i, err := strconv.Atoi(n)
		assert.Nil(ts.T, err, "Atoi")
		_, ok := files[i]
		assert.Equal(ts.T, false, ok, "map")
		files[i] = true
	}
	for i := 0; i < NFILE; i++ {
		assert.Equal(ts.T, true, files[i], "checkFs")
	}
}

func TestConcurRename(t *testing.T) {
	const N = 20
	ts := test.MakeTstatePath(t, path)
	cont := make(chan bool)
	done := make(chan int)
	initfs(ts)

	// start N threads trying to rename files in todo dir
	for i := 0; i < N; i++ {
		fsl := fslib.MakeFsLibAddr("thread"+strconv.Itoa(i), fslib.Named())
		go func(fsl *fslib.FsLib, t string) {
			n := 0
			for c := true; c; {
				select {
				case c = <-cont:
				default:
					n += testRename(ts, fsl, t)
				}
			}
			done <- n
		}(fsl, strconv.Itoa(i))
	}

	// generate files in the todo dir
	for i := 0; i < NFILE; i++ {
		_, err := ts.PutFile(TODO+"/job"+strconv.Itoa(i), 07000, np.OWRITE, []byte{})
		assert.Nil(ts.T, err, "Create job")
	}

	// tell threads we are done with generating files
	n := 0
	for i := 0; i < N; i++ {
		cont <- false
		n += <-done
	}
	assert.Equal(ts.T, NFILE, n, "sum")
	checkFs(ts)
	ts.Shutdown()
}

func TestPipeSimple(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	err := ts.MakePipe("name/pipe", 0777)
	assert.Nil(ts.T, err, "MakePipe")

	ch := make(chan bool)
	go func() {
		fsl := fslib.MakeFsLibAddr("reader", fslib.Named())
		fd, err := fsl.Open("name/pipe", np.OREAD)
		assert.Nil(ts.T, err, "Open")
		b, err := fsl.Read(fd, 100)
		assert.Nil(ts.T, err, "Read")
		assert.Equal(ts.T, "hello", string(b))
		err = fsl.Close(fd)
		assert.Nil(ts.T, err, "Close")
		ch <- true
	}()
	fd, err := ts.Open("name/pipe", np.OWRITE)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Write(fd, []byte("hello"))
	assert.Nil(ts.T, err, "Write")
	err = ts.Close(fd)
	assert.Nil(ts.T, err, "Close")

	<-ch

	ts.Shutdown()
}

func TestPipeClose(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	err := ts.MakePipe("name/pipe", 0777)
	assert.Nil(ts.T, err, "MakePipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		fsl := fslib.MakeFsLibAddr("reader", fslib.Named())
		fd, err := fsl.Open("name/pipe", np.OREAD)
		assert.Nil(ts.T, err, "Open")
		for true {
			b, err := fsl.Read(fd, 100)
			if err != nil { // writer closed pipe
				break
			}
			assert.Nil(ts.T, err, "Read")
			assert.Equal(ts.T, "hello", string(b))
		}
		err = fsl.Close(fd)
		assert.Nil(ts.T, err, "Close")
		ch <- true
	}(ch)
	fd, err := ts.Open("name/pipe", np.OWRITE)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Write(fd, []byte("hello"))
	assert.Nil(ts.T, err, "Write")
	err = ts.Close(fd)
	assert.Nil(ts.T, err, "Close")

	<-ch

	ts.Shutdown()
}

func TestPipeRemove(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	err := ts.MakePipe("name/pipe", 0777)
	assert.Nil(ts.T, err, "MakePipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		fsl := fslib.MakeFsLibAddr("reader", fslib.Named())
		_, err := fsl.Open("name/pipe", np.OREAD)
		assert.NotNil(ts.T, err, "Open")
		ch <- true
	}(ch)
	time.Sleep(500 * time.Millisecond)
	err = ts.Remove("name/pipe")
	assert.Nil(ts.T, err, "Remove")

	<-ch

	ts.Shutdown()
}

func TestPipeCrash0(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	err := ts.MakePipe("name/pipe", 0777)
	assert.Nil(ts.T, err, "MakePipe")

	go func() {
		fsl := fslib.MakeFsLibAddr("writer", fslib.Named())
		_, err := fsl.Open("name/pipe", np.OWRITE)
		assert.Nil(ts.T, err, "Open")
		time.Sleep(200 * time.Millisecond)
		// simulate thread crashing
		fsl.Disconnect("name/pipe")
	}()
	fd, err := ts.Open("name/pipe", np.OREAD)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Read(fd, 100)
	assert.NotNil(ts.T, err, "read")
	ts.Shutdown()
}

func TestPipeCrash1(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	err := ts.MakePipe("name/pipe", 0777)
	assert.Nil(ts.T, err, "MakePipe")

	fsl1 := fslib.MakeFsLibAddr("w1", fslib.Named())
	go func() {
		// blocks
		_, err := fsl1.Open("name/pipe", np.OWRITE)
		assert.NotNil(ts.T, err, "Open")
	}()

	time.Sleep(200 * time.Millisecond)

	// simulate crash of w1
	fsl1.Disconnect("name/pipe")

	// start up second write to pipe
	go func() {
		fsl2 := fslib.MakeFsLibAddr("w2", fslib.Named())
		// the pipe has been closed for writing due to crash;
		// this open should fail.
		_, err := fsl2.Open("name/pipe", np.OWRITE)
		assert.NotNil(ts.T, err, "Open")
	}()

	time.Sleep(200 * time.Millisecond)

	fd, err := ts.Open("name/pipe", np.OREAD)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Read(fd, 100)
	assert.NotNil(ts.T, err, "read")

	ts.Shutdown()
}

func TestSymlinkPath(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	dn := "name/d"
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink([]byte("name/"), "name/namedself", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir("name/namedself/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"d", "namedself"}), "dir")

	ts.Shutdown()
}

func TestSymlinkRemote(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	dn := "name/d"
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink(fslib.MakeTarget(fslib.Named()), "name/namedself", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir("name/namedself/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"d", "namedself"}), "dir")

	ts.Shutdown()
}

func TestUnionDir(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	dn := "name/d"
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink(fslib.MakeTarget(fslib.Named()), "name/d/namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")
	err = ts.Symlink(fslib.MakeTarget(np.Path{":2222"}), "name/d/namedself1", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir("name/d/~ip/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"d"}), "dir")

	sts, err = ts.GetDir("name/d/~ip/d/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"namedself0", "namedself1"}), "dir")

	ts.Shutdown()
}

func TestUnionRoot(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	err := ts.Symlink(fslib.MakeTarget(fslib.Named()), "name/namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")
	err = ts.Symlink(fslib.MakeTarget(np.Path{"xxx"}), "name/namedself1", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir("name/~ip/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"namedself0", "namedself1"}), "dir")

	ts.Shutdown()
}

func TestUnionSymlinkRead(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	err := ts.Symlink(fslib.MakeTarget(fslib.Named()), "name/namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	dn := "name/d"
	err = ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")
	err = ts.Symlink(fslib.MakeTarget(fslib.Named()), "name/d/namedself1", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir("name/~ip/d/namedself1/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"statsd", "d", "namedself0"}), "root wrong")

	sts, err = ts.GetDir("name/~ip/d/namedself1/d/")
	assert.Equal(t, nil, err)
	log.Printf("sts %v\n", sts)
	assert.True(t, fslib.Present(sts, np.Path{"namedself1"}), "d wrong")

	ts.Shutdown()
}

func TestUnionSymlinkPut(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	err := ts.Symlink(fslib.MakeTarget(fslib.Named()), "name/namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	b := []byte("hello")
	fn := "name/~ip/namedself0/f"
	_, err = ts.PutFile(fn, 0777, np.OWRITE, b)
	assert.Equal(t, nil, err)

	fn1 := "name/~ip/namedself0/g"
	_, err = ts.PutFile(fn1, 0777, np.OWRITE, b)
	assert.Equal(t, nil, err)

	sts, err := ts.GetDir("name/~ip/namedself0/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"statsd", "f", "g"}), "root wrong")

	d, err := ts.GetFile("name/~ip/namedself0/f")
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, b, d, "GetFile")

	d, err = ts.GetFile("name/~ip/namedself0/g")
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, b, d, "GetFile")

	ts.Shutdown()
}

func TestSetFileSymlink(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := "name/f"
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	ts.Symlink(fslib.MakeTarget(fslib.Named()), "name/namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	st := stats.StatInfo{}
	err = ts.GetFileJson("name/statsd", &st)
	assert.Nil(t, err, "statsd")
	nwalk := st.Nwalk

	d = []byte("byebye")
	n, err := ts.SetFile("name/namedself0/f", d, 0)
	assert.Nil(ts.T, err, "SetFile")
	assert.Equal(ts.T, np.Tsize(len(d)), n, "SetFile")

	err = ts.GetFileJson("name/statsd", &st)
	assert.Nil(t, err, "statsd")

	assert.NotEqual(ts.T, nwalk, st.Nwalk, "setfile")
	nwalk = st.Nwalk

	b, err := ts.GetFile("name/namedself0/f")
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, d, b, "GetFile")

	err = ts.GetFileJson("name/statsd", &st)
	assert.Nil(t, err, "statsd")

	assert.Equal(ts.T, nwalk, st.Nwalk, "getfile")

	ts.Shutdown()
}

func TestOpenRemoveRead(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := "name/f"
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	b, err := rdr.GetData()
	assert.Equal(t, nil, err)
	assert.Equal(t, d, b, "data")

	rdr.Close()

	_, err = ts.Stat(fn)
	assert.NotNil(t, err, "stat")

	ts.Shutdown()
}
