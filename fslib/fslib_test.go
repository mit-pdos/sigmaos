package fslib_test

import (
	"bufio"
	"flag"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/klauspost/readahead"
	"github.com/stretchr/testify/assert"

	"ulambda/awriter"
	db "ulambda/debug"
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

func TestRemoveBasic(t *testing.T) {
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

func TestCreateTwice(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := path + "f"
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)
	_, err = ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.NotNil(t, err)
	assert.True(t, np.IsErrExists(err))
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

	srv, _, err := ts.PathServer(path)
	assert.Nil(t, err)

	err = ts.Disconnect(srv)
	assert.Nil(t, err, "Disconnect")
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

	err = ts.Remove(path + "this-file-does-not-exist")
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

func TestReadOff(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := path + "/f"
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)

	rdr.Lseek(3)
	b := make([]byte, 10)
	n, err := rdr.Read(b)
	assert.Nil(t, err)
	assert.Equal(t, 2, n)

	ts.Shutdown()
}

func TestRenameBasic(t *testing.T) {
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
	d1 := path + "/d1/"
	d2 := path + "/d2/"
	ts := test.MakeTstatePath(t, path)
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir(d2, 0777)
	assert.Equal(t, nil, err)

	fn := d1 + "/f"
	fn1 := d2 + "/g"
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Rename(fn, fn1)
	assert.Equal(t, nil, err)

	b, err := ts.GetFile(fn1)
	assert.Equal(t, nil, err)
	assert.Equal(t, "hello", string(b))

	_, err = ts.Stat(fn1)
	assert.Equal(t, nil, err)

	err = ts.Remove(fn1)
	assert.Equal(t, nil, err)
	ts.Shutdown()
}

func TestNonEmpty(t *testing.T) {
	d1 := path + "/d1/"
	d2 := path + "/d2/"

	ts := test.MakeTstatePath(t, path)
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir(d2, 0777)
	assert.Equal(t, nil, err)

	fn := d1 + "f"
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove(d1)
	assert.NotNil(t, err, "Remove")

	err = ts.Rename(d2, d1)
	assert.NotNil(t, err, "Rename")

	ts.Shutdown()
}

func TestSetAppend(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	d := []byte("1234")
	fn := path + "f"

	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)
	l, err := ts.SetFile(fn, d, np.OAPPEND, np.NoOffset)
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
	src := path + "f"
	dst := path + "g"
	_, err := ts.PutFile(src, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.CopyFile(src, dst)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(dst)
	assert.Equal(t, "hello", string(d1))

	ts.Shutdown()
}

func TestDirBasic(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	dn := path + "d/"
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
	assert.NotNil(t, err)

	ts.Shutdown()
}

func TestDirDot(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	dn := path + "dir0/"
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	b, err := ts.IsDir(dn + "/.")
	assert.Equal(t, nil, err)
	assert.Equal(t, true, b)
	err = ts.RmDir(dn + "/.")
	assert.NotNil(t, err)
	err = ts.RmDir(dn)
	_, err = ts.Stat(dn + "/.")
	assert.NotEqual(t, nil, err)
	_, err = ts.Stat(path + "/.")
	assert.Equal(t, nil, err)
	ts.Shutdown()
}

func TestPageDir(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	dn := path + "/dir/"
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	ts.SetChunkSz(np.Tsize(512))
	n := 1000
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

func dirwriter(t *testing.T, dn string, name string, ch chan bool) {
	fsl := fslib.MakeFsLibAddr("fslibtest-"+name, fslib.Named())
	stop := false
	for !stop {
		select {
		case stop = <-ch:
		default:
			err := fsl.Remove(dn + name)
			assert.Nil(t, err)
			_, err = fsl.PutFile(dn+name, 0777, np.OWRITE, []byte(name))
			assert.Nil(t, err)
		}
	}
}

// Concurrently scan dir and create/remove entries
func TestDirConcur(t *testing.T) {
	const (
		N     = 1
		NFILE = 3
		NSCAN = 100
	)
	ts := test.MakeTstatePath(t, path)
	dn := path + "/dir/"
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)

	for i := 0; i < NFILE; i++ {
		name := strconv.Itoa(i)
		_, err := ts.PutFile(dn+name, 0777, np.OWRITE, []byte(name))
		assert.Equal(t, nil, err)
	}

	ch := make(chan bool)
	for i := 0; i < N; i++ {
		go dirwriter(t, dn, strconv.Itoa(i), ch)
	}

	for i := 0; i < NSCAN; i++ {
		i := 0
		names := []string{}
		b, err := ts.ProcessDir(dn, func(st *np.Stat) (bool, error) {
			names = append(names, st.Name)
			i += 1
			return false, nil

		})
		assert.Nil(t, err)
		assert.False(t, b)

		if i < NFILE-N {
			log.Printf("names %v\n", names)
		}

		assert.True(t, i >= NFILE-N)

		uniq := make(map[string]bool)
		for _, n := range names {
			if _, ok := uniq[n]; ok {
				assert.True(t, n == strconv.Itoa(NFILE-1))
			}
			uniq[n] = true
		}
	}

	for i := 0; i < N; i++ {
		ch <- true
	}

	ts.Shutdown()
}

func readWrite(t *testing.T, fsl *fslib.FsLib, cnt string) bool {
	fd, err := fsl.Open(cnt, np.ORDWR)
	assert.Nil(t, err)

	defer fsl.Close(fd)

	b, err := fsl.ReadV(fd, 1000)
	if err != nil && np.IsErrVersion(err) {
		return true
	}
	assert.Nil(t, err)
	n, err := strconv.Atoi(string(b))
	assert.Nil(t, err)

	n += 1

	err = fsl.Seek(fd, 0)
	assert.Nil(t, err)

	b = []byte(strconv.Itoa(n))
	_, err = fsl.WriteV(fd, b)
	if err != nil && np.IsErrVersion(err) {
		return true
	}
	assert.Nil(t, err)

	return false
}

func TestCounter(t *testing.T) {
	const N = 10

	ts := test.MakeTstatePath(t, path)
	cnt := path + "cnt"
	b := []byte(strconv.Itoa(0))
	_, err := ts.PutFile(cnt, 0777|np.DMTMP, np.OWRITE, b)
	assert.Equal(t, nil, err)

	ch := make(chan int)

	for i := 0; i < N; i++ {
		go func(i int) {
			ntrial := 0
			for {
				ntrial += 1
				if readWrite(t, ts.FsLib, cnt) {
					continue
				}
				break
			}
			// log.Printf("%d: tries %v\n", i, ntrial)
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
	}
	b, err = ts.GetFile(cnt)
	assert.Equal(t, nil, err)
	n, err := strconv.Atoi(string(b))
	assert.Equal(t, nil, err)

	assert.Equal(t, N, n)

	ts.Shutdown()
}

func TestWatchCreate(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := path + "w"
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

	fn := path + "w"
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

	fn := path + "/d1/"
	err := ts.MkDir(fn, 0777)
	assert.Equal(t, nil, err)

	_, rdr, err := ts.ReadDir(fn)
	assert.Equal(t, nil, err)
	ch := make(chan bool)
	err = ts.SetDirWatch(rdr.Fid(), fn, func(path string, err error) {
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

func TestCreateExcl1(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	ch := make(chan int)

	fn := path + "exclusive"
	_, err := ts.PutFile(fn, 0777|np.DMTMP, np.OWRITE|np.OCEXEC, []byte{})
	assert.Equal(t, nil, err)
	fsl := fslib.MakeFsLibAddr("fslibtest0", fslib.Named())
	go func() {
		_, err := fsl.PutFile(fn, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
		assert.Nil(t, err, "Putfile")
		ch <- 0
	}()
	time.Sleep(time.Second * 2)
	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove")
	go func() {
		time.Sleep(2 * time.Second)
		ch <- 1
	}()
	i := <-ch
	assert.Equal(t, 0, i)

	ts.Shutdown()
}

func TestCreateExclN(t *testing.T) {
	const N = 20

	ts := test.MakeTstatePath(t, path)
	ch := make(chan int)
	fn := path + "exclusive"
	acquired := false
	for i := 0; i < N; i++ {
		go func(i int) {
			fsl := fslib.MakeFsLibAddr("fslibtest"+strconv.Itoa(i), fslib.Named())
			_, err := fsl.PutFile(fn, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
			assert.Equal(t, nil, err)
			assert.Equal(t, false, acquired)
			acquired = true
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
		acquired = false
		err := ts.Remove(fn)
		assert.Equal(t, nil, err)
	}
	ts.Shutdown()
}

func TestCreateExclAfterDisconnect(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := path + "create-conn-close-test"

	fsl1 := fslib.MakeFsLibAddr("fslibtest-1", fslib.Named())

	_, err := ts.PutFile(fn, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	assert.Nil(t, err, "Create 1")

	go func() {
		// Should wait
		_, err := fsl1.PutFile(fn, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
		assert.NotNil(t, err, "Create 2")
	}()

	time.Sleep(500 * time.Millisecond)

	// Kill fsl1's connection
	srv, _, err := ts.PathServer(path)
	assert.Nil(t, err)

	db.DPrintf("TEST", "Disconnect fsl")
	err = fsl1.Disconnect(srv)
	assert.Nil(t, err, "Disconnect")

	// Remove the ephemeral file
	ts.Remove(fn)
	assert.Equal(t, nil, err)

	// Try to create again (should succeed)
	_, err = ts.PutFile(fn, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	assert.Nil(t, err, "Create 3")

	ts.Shutdown()
}

func TestWatchRemoveConcur(t *testing.T) {
	const N = 5_000

	ts := test.MakeTstatePath(t, path)
	dn := path + "d1/"
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
				fn := path + "f" + strconv.Itoa(i)
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
	NFILE = 1000
)

func initfs(ts *test.Tstate, TODO, DONE string) {
	err := ts.MkDir(TODO, 07777)
	assert.Nil(ts.T, err, "Create done")
	err = ts.MkDir(DONE, 07777)
	assert.Nil(ts.T, err, "Create todo")
}

// Keep renaming files in the todo directory until we failed to rename
// any file
func testRename(ts *test.Tstate, fsl *fslib.FsLib, t string, TODO, DONE string) int {
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

func checkFs(ts *test.Tstate, DONE string) {
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
	TODO := path + "todo"
	DONE := path + "done"

	initfs(ts, TODO, DONE)

	// start N threads trying to rename files in todo dir
	for i := 0; i < N; i++ {
		fsl := fslib.MakeFsLibAddr("thread"+strconv.Itoa(i), fslib.Named())
		go func(fsl *fslib.FsLib, t string) {
			n := 0
			for c := true; c; {
				select {
				case c = <-cont:
				default:
					n += testRename(ts, fsl, t, TODO, DONE)
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
	checkFs(ts, DONE)
	ts.Shutdown()
}

func TestPipeBasic(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	pipe := path + "pipe"
	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	ch := make(chan bool)
	go func() {
		fsl := fslib.MakeFsLibAddr("reader", fslib.Named())
		fd, err := fsl.Open(pipe, np.OREAD)
		assert.Nil(ts.T, err, "Open")
		b, err := fsl.Read(fd, 100)
		assert.Nil(ts.T, err, "Read")
		assert.Equal(ts.T, "hello", string(b))
		err = fsl.Close(fd)
		assert.Nil(ts.T, err, "Close")
		ch <- true
	}()
	fd, err := ts.Open(pipe, np.OWRITE)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Write(fd, []byte("hello"))
	assert.Nil(ts.T, err, "Write")
	err = ts.Close(fd)
	assert.Nil(ts.T, err, "Close")

	<-ch

	ts.Remove(pipe)

	ts.Shutdown()
}

func TestPipeClose(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	pipe := path + "pipe"
	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		fsl := fslib.MakeFsLibAddr("reader", fslib.Named())
		fd, err := fsl.Open(pipe, np.OREAD)
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
		assert.Nil(ts.T, err, "Close: %v", err)
		ch <- true
	}(ch)
	fd, err := ts.Open(pipe, np.OWRITE)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Write(fd, []byte("hello"))
	assert.Nil(ts.T, err, "Write")
	err = ts.Close(fd)
	assert.Nil(ts.T, err, "Close")

	<-ch

	ts.Remove(pipe)

	ts.Shutdown()
}

func TestPipeRemove(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	pipe := path + "pipe"

	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	ch := make(chan bool)
	go func(ch chan bool) {
		fsl := fslib.MakeFsLibAddr("reader", fslib.Named())
		_, err := fsl.Open(pipe, np.OREAD)
		assert.NotNil(ts.T, err, "Open")
		ch <- true
	}(ch)
	time.Sleep(500 * time.Millisecond)
	err = ts.Remove(pipe)
	assert.Nil(ts.T, err, "Remove")

	<-ch

	ts.Shutdown()
}

func TestPipeCrash0(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	pipe := path + "pipe"
	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	go func() {
		fsl := fslib.MakeFsLibAddr("writer", fslib.Named())
		_, err := fsl.Open(pipe, np.OWRITE)
		assert.Nil(ts.T, err, "Open")
		time.Sleep(200 * time.Millisecond)
		// simulate thread crashing
		srv, _, err := ts.PathServer(path)
		assert.Nil(t, err)
		err = fsl.Disconnect(srv)
		assert.Nil(ts.T, err, "Disconnect")

	}()
	fd, err := ts.Open(pipe, np.OREAD)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Read(fd, 100)
	assert.NotNil(ts.T, err, "read")

	ts.Remove(pipe)
	ts.Shutdown()
}

func TestPipeCrash1(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	pipe := path + "pipe"
	err := ts.MakePipe(pipe, 0777)
	assert.Nil(ts.T, err, "MakePipe")

	fsl1 := fslib.MakeFsLibAddr("w1", fslib.Named())
	go func() {
		// blocks
		_, err := fsl1.Open(pipe, np.OWRITE)
		assert.NotNil(ts.T, err, "Open")
	}()

	time.Sleep(200 * time.Millisecond)

	// simulate crash of w1
	srv, _, err := ts.PathServer(path)
	assert.Nil(t, err)
	err = fsl1.Disconnect(srv)
	assert.Nil(ts.T, err, "Disconnect")

	time.Sleep(2 * np.Conf.Session.TIMEOUT)

	// start up second write to pipe
	go func() {
		fsl2 := fslib.MakeFsLibAddr("w2", fslib.Named())
		// the pipe has been closed for writing due to crash;
		// this open should fail.
		_, err := fsl2.Open(pipe, np.OWRITE)
		assert.NotNil(ts.T, err, "Open")
	}()

	time.Sleep(200 * time.Millisecond)

	fd, err := ts.Open(pipe, np.OREAD)
	assert.Nil(ts.T, err, "Open")
	_, err = ts.Read(fd, 100)
	assert.NotNil(ts.T, err, "read")

	ts.Remove(pipe)
	ts.Shutdown()
}

func TestSymlinkPath(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	dn := path + "d"
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink([]byte(path), path+"namedself", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir(path + "namedself/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"d", "namedself"}), "dir")

	ts.Shutdown()
}

func target(t *testing.T, ts *test.Tstate, path string) []byte {
	target := fslib.MakeTarget(fslib.Named())
	if path != np.NAMED {
		srv, left, err := ts.AbsPathServer(path)
		assert.Nil(t, err)
		target = fslib.MakeTargetTree(srv.Base(), left)
	}
	return target
}

func TestSymlinkRemote(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	dn := path + "d"
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink(target(t, ts, path), path+"namedself", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir(path + "namedself/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"d", "namedself"}), "dir")

	ts.Shutdown()
}

func TestUnionDir(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	dn := path + "d"
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink(target(t, ts, path), path+"d/namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")
	err = ts.Symlink(fslib.MakeTarget(np.Path{":2222"}), path+"d/namedself1", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir(path + "/d/~any/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"d"}), "dir")

	sts, err = ts.GetDir(path + "d/~any/d/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"namedself0", "namedself1"}), "dir")

	ts.Shutdown()
}

func TestUnionRoot(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	err := ts.Symlink(target(t, ts, path), path+"namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")
	err = ts.Symlink(fslib.MakeTarget(np.Path{"xxx"}), path+"namedself1", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir(path + "~any/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"namedself0", "namedself1"}), "dir")

	ts.Shutdown()
}

func TestUnionSymlinkRead(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	err := ts.Symlink(target(t, ts, path), path+"namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	dn := path + "/d/"
	err = ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")
	err = ts.Symlink(target(t, ts, path), path+"d/namedself1", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir(path + "~any/d/namedself1/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"d", "namedself0"}), "root wrong")

	sts, err = ts.GetDir(path + "~any/d/namedself1/d/")
	assert.Equal(t, nil, err)
	log.Printf("sts %v\n", sts)
	assert.True(t, fslib.Present(sts, np.Path{"namedself1"}), "d wrong")

	ts.Shutdown()
}

func TestUnionSymlinkPut(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	err := ts.Symlink(target(t, ts, path), path+"namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	b := []byte("hello")
	fn := path + "~any/namedself0/f"
	_, err = ts.PutFile(fn, 0777, np.OWRITE, b)
	assert.Equal(t, nil, err)

	fn1 := path + "~any/namedself0/g"
	_, err = ts.PutFile(fn1, 0777, np.OWRITE, b)
	assert.Equal(t, nil, err)

	sts, err := ts.GetDir(path + "~any/namedself0/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, np.Path{"f", "g"}), "root wrong")

	d, err := ts.GetFile(path + "~any/namedself0/f")
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, b, d, "GetFile")

	d, err = ts.GetFile(path + "~any/namedself0/g")
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, b, d, "GetFile")

	ts.Shutdown()
}

func TestSetFileSymlink(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := path + "f"
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	ts.Symlink(target(t, ts, path), path+"namedself0", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	st := stats.StatInfo{}
	err = ts.GetFileJson("name/"+np.STATSD, &st)
	assert.Nil(t, err, "statsd")
	nwalk := st.Nwalk

	d = []byte("byebye")
	n, err := ts.SetFile(path+"namedself0/f", d, np.OWRITE, 0)
	assert.Nil(ts.T, err, "SetFile: %v", err)
	assert.Equal(ts.T, np.Tsize(len(d)), n, "SetFile")

	err = ts.GetFileJson(path+"/"+np.STATSD, &st)
	assert.Nil(t, err, "statsd")

	assert.NotEqual(ts.T, nwalk, st.Nwalk, "setfile")
	nwalk = st.Nwalk

	b, err := ts.GetFile(path + "namedself0/f")
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, d, b, "GetFile")

	err = ts.GetFileJson(path+"/"+np.STATSD, &st)
	assert.Nil(t, err, "statsd")

	assert.Equal(ts.T, nwalk, st.Nwalk, "getfile")

	ts.Shutdown()
}

func TestOpenRemoveRead(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	fn := path + "f"
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

func TestFslibExit(t *testing.T) {
	ts := test.MakeTstatePath(t, path)

	// connect
	_, err := ts.Stat(path + "/.")
	assert.Nil(t, err)

	// close
	err = ts.Exit()
	assert.Nil(t, err)

	_, err = ts.Stat(path + "/.")
	assert.NotNil(t, err)
	assert.True(t, np.IsErrUnreachable(err))

	ts.Shutdown()
}

const (
	KBYTE      = 1 << 10
	NRUNS      = 1
	SYNCFILESZ = 100 * KBYTE
	FILESZ     = 20 * test.MBYTE
	WRITESZ    = 4096
)

func measure(msg string, f func() np.Tlength) {
	for i := 0; i < NRUNS; i++ {
		start := time.Now()
		sz := f()
		ms := time.Since(start).Milliseconds()
		log.Printf("%v: %s took %vms (%s)", msg, humanize.Bytes(uint64(sz)), ms, test.Tput(sz, ms))
	}
}

func measuredir(msg string, nruns int, f func() int) {
	tot := float64(0)
	n := 0
	for i := 0; i < nruns; i++ {
		start := time.Now()
		n += f()
		ms := time.Since(start).Milliseconds()
		tot += float64(ms)
	}
	s := tot / 1000
	log.Printf("%v: %d entries took %vms (%.1f file/s)", msg, n, tot, float64(n)/s)
}

type Thow uint8

const (
	HSYNC Thow = iota + 1
	HBUF
	HASYNC
)

func mkFile(t *testing.T, fsl *fslib.FsLib, fn string, how Thow, buf []byte, sz np.Tlength) np.Tlength {
	w, err := fsl.CreateWriter(fn, 0777, np.OWRITE)
	assert.Nil(t, err)
	switch how {
	case HSYNC:
		err = test.Writer(t, w, buf, sz)
		assert.Nil(t, err)
	case HBUF:
		bw := bufio.NewWriterSize(w, test.BUFSZ)
		err = test.Writer(t, bw, buf, sz)
		assert.Nil(t, err)
		err = bw.Flush()
		assert.Nil(t, err)
	case HASYNC:
		aw := awriter.NewWriterSize(w, test.BUFSZ)
		bw := bufio.NewWriterSize(aw, test.BUFSZ)
		err = test.Writer(t, bw, buf, sz)
		assert.Nil(t, err)
		err = bw.Flush()
		assert.Nil(t, err)
		err = aw.Close()
		assert.Nil(t, err)
	}
	w.Close()
	st, err := fsl.Stat(fn)
	assert.Nil(t, err)
	assert.Equal(t, np.Tlength(sz), st.Length, "stat")
	return sz
}

func TestWriteFilePerf(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	fn := path + "f"
	buf := test.MkBuf(WRITESZ)
	measure("writer", func() np.Tlength {
		sz := mkFile(t, ts.FsLib, fn, HSYNC, buf, SYNCFILESZ)
		err := ts.Remove(fn)
		assert.Nil(t, err)
		return sz
	})
	measure("bufwriter", func() np.Tlength {
		sz := mkFile(t, ts.FsLib, fn, HBUF, buf, FILESZ)
		err := ts.Remove(fn)
		assert.Nil(t, err)
		return sz
	})
	measure("abufwriter", func() np.Tlength {
		sz := mkFile(t, ts.FsLib, fn, HASYNC, buf, FILESZ)
		err := ts.Remove(fn)
		assert.Nil(t, err)
		return sz
	})
	ts.Shutdown()
}

func TestReadFilePerf(t *testing.T) {
	ts := test.MakeTstatePath(t, path)
	fn := path + "f"
	buf := test.MkBuf(WRITESZ)
	sz := mkFile(t, ts.FsLib, fn, HBUF, buf, SYNCFILESZ)
	measure("reader", func() np.Tlength {
		r, err := ts.OpenReader(fn)
		assert.Nil(t, err)
		n, err := test.Reader(t, r, buf, sz)
		assert.Nil(t, err)
		return n
	})
	err := ts.Remove(fn)
	assert.Nil(t, err)
	sz = mkFile(t, ts.FsLib, fn, HBUF, buf, FILESZ)
	measure("bufreader", func() np.Tlength {
		r, err := ts.OpenReader(fn)
		assert.Nil(t, err)
		br := bufio.NewReaderSize(r, test.BUFSZ)
		n, err := test.Reader(t, br, buf, sz)
		assert.Nil(t, err)
		return n
	})
	measure("readahead", func() np.Tlength {
		r, err := ts.OpenReader(fn)
		assert.Nil(t, err)
		br, err := readahead.NewReaderSize(r, 4, test.BUFSZ)
		assert.Nil(t, err)
		n, err := test.Reader(t, br, buf, sz)
		assert.Nil(t, err)
		return n
	})
	err = ts.Remove(fn)
	assert.Nil(t, err)
	ts.Shutdown()
}

func mkDir(t *testing.T, fsl *fslib.FsLib, dir string, n int) int {
	err := fsl.MkDir(dir, 0777)
	assert.Equal(t, nil, err)
	for i := 0; i < n; i++ {
		b := []byte("hello")
		_, err := fsl.PutFile(dir+"/f"+strconv.Itoa(i), 0777, np.OWRITE, b)
		assert.Nil(t, err)
	}
	return n
}

func TestDirCreatePerf(t *testing.T) {
	const N = 1000
	ts := test.MakeTstatePath(t, path)
	dir := path + "d"
	measuredir("create dir", 1, func() int {
		n := mkDir(t, ts.FsLib, dir, N)
		return n
	})
	err := ts.RmDir(dir)
	assert.Nil(t, err)
	ts.Shutdown()
}

func lookuper(t *testing.T, nclerk int, n int, dir string, nfile int) {
	const NITER = 10000
	ch := make(chan bool)
	for c := 0; c < nclerk; c++ {
		go func() {
			cn := strconv.Itoa(c)
			fsl := fslib.MakeFsLibAddr("fslibtest-"+cn, fslib.Named())
			measuredir("lookup dir entry", NITER, func() int {
				for f := 0; f < nfile; f++ {
					_, err := fsl.Stat(dir + "/f" + strconv.Itoa(f))
					assert.Nil(t, err)
				}
				return nfile
			})
			ch <- true
		}()
	}
	for c := 0; c < nclerk; c++ {
		<-ch
	}
}

func TestDirReadPerf(t *testing.T) {
	const N = 10000
	const NFILE = 10
	const NCLERK = 1
	ts := test.MakeTstatePath(t, path)
	dir := path + "d"
	n := mkDir(t, ts.FsLib, dir, NFILE)
	assert.Equal(t, NFILE, n)
	// measuredir("read dir", 1, func() int {
	// 	n := 0
	// 	ts.ProcessDir(dir, func(st *np.Stat) (bool, error) {
	// 		n += 1
	// 		return false, nil
	// 	})
	// 	return n
	// })
	// lookuper(t, 1, N, dir)
	lookuper(t, NCLERK, N, dir, NFILE)
	err := ts.RmDir(dir)
	assert.Nil(t, err)
	ts.Shutdown()
}

func TestRmDirPerf(t *testing.T) {
	const N = 5000
	ts := test.MakeTstatePath(t, path)
	dir := path + "d"
	n := mkDir(t, ts.FsLib, dir, N)
	assert.Equal(t, N, n)
	measuredir("rm dir", 1, func() int {
		err := ts.RmDir(dir)
		assert.Nil(t, err)
		return N
	})
	ts.Shutdown()
}
