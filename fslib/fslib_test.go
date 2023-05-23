package fslib_test

import (
	"errors"
	"flag"
	"log"
	"net"
	gopath "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/named"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/stats"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/~local/fslibtest"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func TestInitFs(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	sts, err := ts.GetDir(pathname)
	assert.Nil(t, err)
	if pathname == sp.NAMED {
		assert.True(t, fslib.Present(sts, named.InitRootDir), "initfs")
		sts, err = ts.GetDir(pathname + "/boot")
		assert.Nil(t, err)
		log.Printf("named %v\n", sp.Names(sts))
	} else {
		log.Printf("%v %v\n", pathname, sp.Names(sts))
		assert.True(t, len(sts) >= 2, "initfs")
	}
	ts.Shutdown()
}

func TestRemoveBasic(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	_, err = ts.Stat(fn)
	assert.NotEqual(t, nil, err)

	ts.Shutdown()
}

func TestRemoveDir(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	db.DPrintf(db.TEST, "path %v", pathname)
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)

	_, err = ts.PutFile(gopath.Join(d1, "f"), 0777, sp.OWRITE, []byte("hello"))
	assert.Equal(t, nil, err)

	sts, err := ts.GetDir(d1 + "/")
	assert.Nil(t, err, "GetDir: %v", err)

	assert.True(t, fslib.Present(sts, []string{"f"}))

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestDirBasic(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	dn := gopath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	b, err := ts.IsDir(dn)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, b)

	d := []byte("hello")
	_, err = ts.PutFile(gopath.Join(dn, "f"), 0777, sp.OWRITE, d)
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

func TestCreateTwice(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Nil(t, err)
	_, err = ts.PutFile(fn, 0777, sp.OWRITE|sp.OEXCL, d)
	assert.NotNil(t, err)
	var serr *serr.Err
	assert.True(t, errors.As(err, &serr) && serr.IsErrExists())

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestConnect(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	fd, err := ts.Create(fn, 0777, sp.OWRITE)
	assert.Equal(t, nil, err)
	_, err = ts.Write(fd, d)
	assert.Equal(t, nil, err)

	srv, _, err := ts.PathLastSymlink(pathname)
	assert.Nil(t, err)

	err = ts.Disconnect(srv)
	assert.Nil(t, err, "Disconnect")
	time.Sleep(100 * time.Millisecond)
	db.DPrintf(db.TEST, "disconnected")

	var serr *serr.Err
	_, err = ts.Write(fd, d)
	assert.True(t, errors.As(err, &serr) && serr.IsErrUnreachable())

	err = ts.Close(fd)
	assert.True(t, errors.As(err, &serr) && serr.IsErrUnreachable())

	fd, err = ts.Open(fn, sp.OREAD)
	assert.True(t, errors.As(err, &serr) && serr.IsErrUnreachable())

	ts.Shutdown()
}

func TestRemoveNonExistent(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove(gopath.Join(pathname, "this-file-does-not-exist"))
	assert.NotNil(t, err)

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestRemovePath(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	fn := gopath.Join(d1, "f")
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	b, err := ts.GetFile(fn)
	assert.Equal(t, "hello", string(b))

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestRenameInDir(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	from := gopath.Join(d1, "f")

	d := []byte("hello")
	_, err = ts.PutFile(from, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	to := gopath.Join(d1, "g")
	err = ts.Rename(from, to)

	sts, err := ts.GetDir(d1)
	assert.Nil(t, err, "GetDir: %v", err)
	assert.True(t, fslib.Present(sts, []string{"g"}))
	b, err := ts.GetFile(to)
	assert.Equal(t, b, d)

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestRemoveSymlink(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	db.DPrintf(db.TEST, "path %v", pathname)
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	fn := gopath.Join(d1, "f")

	mnt := sp.MkMountService(ts.NamedAddr())
	err = ts.MkMountSymlink(fn, mnt)
	assert.Nil(t, err, "MkMount: %v", err)

	sts, err := ts.GetDir(fn + "/")
	assert.Nil(t, err, "GetDir: %v", err)
	assert.True(t, fslib.Present(sts, named.InitRootDir))

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestRmDirWithSymlink(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	fn := gopath.Join(d1, "f")

	mnt := sp.MkMountService(ts.NamedAddr())
	err = ts.MkMountSymlink(fn, mnt)
	assert.Nil(t, err, "MkMount: %v", err)

	sts, err := ts.GetDir(fn + "/")
	assert.Nil(t, err, "GetDir: %v", err)
	assert.True(t, fslib.Present(sts, named.InitRootDir))

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestReadSymlink(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	fn := gopath.Join(d1, "f")

	mnt := sp.MkMountService(ts.NamedAddr())
	err = ts.MkMountSymlink(fn, mnt)
	assert.Nil(t, err, "MkMount: %v", err)

	_, err = ts.GetDir(fn + "/")
	assert.Nil(t, err, "GetDir: %v", err)

	mnt1, err := ts.ReadMount(fn)
	assert.Nil(t, err, "ReadMount: %v", err)

	assert.Equal(t, mnt.Addr[0].Addr, mnt1.Addr[0].Addr)

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestReadOff(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)

	rdr.Lseek(3)
	b := make([]byte, 10)
	n, err := rdr.Read(b)
	assert.Nil(t, err)
	assert.Equal(t, 2, n)

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestRenameAcrossDir(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	d2 := gopath.Join(pathname, "d2")

	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir(d2, 0777)
	assert.Equal(t, nil, err)

	fn := gopath.Join(d1, "f")
	fn1 := gopath.Join(d2, "g")
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Rename(fn, fn1)
	assert.Equal(t, nil, err)

	b, err := ts.GetFile(fn1)
	assert.Equal(t, "hello", string(b))

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	err = ts.RmDir(d2)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestRenameAndRemove(t *testing.T) {
	d1 := gopath.Join(pathname, "d1")
	d2 := gopath.Join(pathname, "d2")
	ts := test.MakeTstatePath(t, pathname)
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir(d2, 0777)
	assert.Equal(t, nil, err)

	fn := gopath.Join(d1, "f")
	fn1 := gopath.Join(d2, "g")
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, d)
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

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	err = ts.RmDir(d2)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestNonEmpty(t *testing.T) {
	d1 := gopath.Join(pathname, "d1")
	d2 := gopath.Join(pathname, "d2")

	ts := test.MakeTstatePath(t, pathname)
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir(d2, 0777)
	assert.Equal(t, nil, err)

	fn := gopath.Join(d1, "f")
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove(d1)
	assert.NotNil(t, err, "Remove")

	err = ts.Rename(d2, d1)
	assert.NotNil(t, err, "Rename")

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	err = ts.RmDir(d2)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestSetAppend(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	d := []byte("1234")
	fn := gopath.Join(pathname, "f")

	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)
	l, err := ts.SetFile(fn, d, sp.OAPPEND, sp.NoOffset)
	assert.Equal(t, nil, err)
	assert.Equal(t, sessp.Tsize(len(d)), l)
	b, err := ts.GetFile(fn)
	assert.Equal(t, nil, err)
	assert.Equal(t, len(d)*2, len(b))

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestCopy(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	d := []byte("hello")
	src := gopath.Join(pathname, "f")
	dst := gopath.Join(pathname, "g")
	_, err := ts.PutFile(src, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.CopyFile(src, dst)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(dst)
	assert.Equal(t, "hello", string(d1))

	err = ts.Remove(src)
	assert.Nil(t, err, "Remove: %v", err)

	err = ts.Remove(dst)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestDirDot(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	dn := gopath.Join(pathname, "dir0")
	dot := dn + "/."
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	b, err := ts.IsDir(dot)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, b)
	err = ts.RmDir(dot)
	assert.NotNil(t, err)
	err = ts.RmDir(dn)
	_, err = ts.Stat(dot)
	assert.NotNil(t, err)
	_, err = ts.Stat(pathname + "/.")
	assert.Nil(t, err, "Couldn't stat %v", err)

	ts.Shutdown()
}

func TestPageDir(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	dn := gopath.Join(pathname, "dir")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	ts.SetChunkSz(sessp.Tsize(512))
	n := 1000
	names := make([]string, 0)
	for i := 0; i < n; i++ {
		name := strconv.Itoa(i)
		names = append(names, name)
		_, err := ts.PutFile(gopath.Join(dn, name), 0777, sp.OWRITE, []byte(name))
		assert.Equal(t, nil, err)
	}
	sort.SliceStable(names, func(i, j int) bool {
		return names[i] < names[j]
	})
	i := 0
	ts.ProcessDir(dn, func(st *sp.Stat) (bool, error) {
		assert.Equal(t, names[i], st.Name)
		i += 1
		return false, nil

	})
	assert.Equal(t, i, n)

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func dirwriter(t *testing.T, dn, name, lip string, nds sp.Taddrs, ch chan bool) {
	fsl, err := fslib.MakeFsLibAddr("fslibtest-"+name, sp.ROOTREALM, lip, nds)
	assert.Nil(t, err)
	stop := false
	for !stop {
		select {
		case stop = <-ch:
		default:
			err := fsl.Remove(gopath.Join(dn, name))
			assert.Nil(t, err)
			_, err = fsl.PutFile(gopath.Join(dn, name), 0777, sp.OWRITE, []byte(name))
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
	ts := test.MakeTstatePath(t, pathname)
	dn := gopath.Join(pathname, "dir")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)

	for i := 0; i < NFILE; i++ {
		name := strconv.Itoa(i)
		_, err := ts.PutFile(gopath.Join(dn, name), 0777, sp.OWRITE, []byte(name))
		assert.Equal(t, nil, err)
	}

	ch := make(chan bool)
	for i := 0; i < N; i++ {
		go dirwriter(t, dn, strconv.Itoa(i), ts.GetLocalIP(), ts.NamedAddr(), ch)
	}

	for i := 0; i < NSCAN; i++ {
		i := 0
		names := []string{}
		b, err := ts.ProcessDir(dn, func(st *sp.Stat) (bool, error) {
			names = append(names, st.Name)
			i += 1
			return false, nil

		})
		assert.Nil(t, err)
		assert.False(t, b)

		if i < NFILE-N {
			db.DPrintf(db.TEST, "names %v", names)
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

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func readWrite(t *testing.T, fsl *fslib.FsLib, cnt string) bool {
	fd, err := fsl.Open(cnt, sp.ORDWR)
	assert.Nil(t, err)

	defer fsl.Close(fd)

	b, err := fsl.ReadV(fd, 1000)
	var serr *serr.Err
	if errors.As(err, &serr) && serr.IsErrVersion() {
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
	if errors.As(err, &serr) && serr.IsErrVersion() {
		return true
	}
	assert.Nil(t, err)

	return false
}

func TestCounter(t *testing.T) {
	const N = 10

	ts := test.MakeTstatePath(t, pathname)
	cnt := gopath.Join(pathname, "cnt")
	b := []byte(strconv.Itoa(0))
	_, err := ts.PutFile(cnt, 0777|sp.DMTMP, sp.OWRITE, b)
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

	err = ts.Remove(cnt)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestWatchCreate(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "w")
	ch := make(chan bool)
	fd, err := ts.OpenWatch(fn, sp.OREAD, func(string, error) {
		ch <- true
	})
	assert.NotEqual(t, nil, err)
	assert.Equal(t, -1, fd, err)
	var serr *serr.Err
	assert.True(t, errors.As(err, &serr) && serr.IsErrNotfound())

	// give Watch goroutine to start
	time.Sleep(100 * time.Millisecond)

	_, err = ts.PutFile(fn, 0777, sp.OWRITE, nil)
	assert.Equal(t, nil, err)

	<-ch

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestWatchRemoveOne(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "w")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, nil)
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
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "d1")
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

	_, err = ts.PutFile(gopath.Join(fn, "x"), 0777, sp.OWRITE, nil)
	assert.Equal(t, nil, err)

	<-ch

	err = ts.RmDir(fn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestCreateExcl1(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)
	ch := make(chan int)

	fn := gopath.Join(pathname, "exclusive")
	_, err := ts.PutFile(fn, 0777|sp.DMTMP, sp.OWRITE|sp.OCEXEC, []byte{})
	assert.Nil(t, err)
	fsl, err := fslib.MakeFsLibAddr("fslibtest0", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
	assert.Nil(t, err)
	go func() {
		_, err := fsl.PutFile(fn, 0777|sp.DMTMP, sp.OWRITE|sp.OWATCH, []byte{})
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

	ts.Remove(fn)

	ts.Shutdown()
}

func TestCreateExclN(t *testing.T) {
	const N = 20

	ts := test.MakeTstatePath(t, pathname)
	ch := make(chan int)
	fn := gopath.Join(pathname, "exclusive")
	acquired := false
	for i := 0; i < N; i++ {
		go func(i int) {
			fsl, err := fslib.MakeFsLibAddr("fslibtest"+strconv.Itoa(i), sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
			assert.Nil(t, err)
			//log.Printf("PutFile %d\n", i)
			_, err = fsl.PutFile(fn, 0777|sp.DMTMP, sp.OWRITE|sp.OWATCH, []byte{})
			assert.Equal(t, nil, err)
			assert.Equal(t, false, acquired)
			//log.Printf("PutFile %d done\n", i)
			acquired = true
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
		//log.Printf("Remove %d\n", i)
		acquired = false
		err := ts.Remove(fn)
		assert.Equal(t, nil, err)
	}
	ts.Shutdown()
}

func TestCreateExclAfterDisconnect(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "create-conn-close-test")

	fsl1, err := fslib.MakeFsLibAddr("fslibtest-1", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
	assert.Nil(t, err)
	_, err = ts.PutFile(fn, 0777|sp.DMTMP, sp.OWRITE|sp.OWATCH, []byte{})
	assert.Nil(t, err, "Create 1")

	go func() {
		// Should wait
		_, err := fsl1.PutFile(fn, 0777|sp.DMTMP, sp.OWRITE|sp.OWATCH, []byte{})
		assert.NotNil(t, err, "Create 2")
	}()

	time.Sleep(500 * time.Millisecond)

	// Kill fsl1's connection
	srv, _, err := ts.PathLastSymlink(pathname)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Disconnect fsl")
	err = fsl1.Disconnect(srv)
	assert.Nil(t, err, "Disconnect")

	// Remove the ephemeral file
	ts.Remove(fn)
	assert.Equal(t, nil, err)

	// Try to create again (should succeed)
	_, err = ts.PutFile(fn, 0777|sp.DMTMP, sp.OWRITE|sp.OWATCH, []byte{})
	assert.Nil(t, err, "Create 3")

	ts.Remove(fn)
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func TestWatchRemoveConcur(t *testing.T) {
	const N = 50 // 5_000
	const MS = 10

	ts := test.MakeTstatePath(t, pathname)
	dn := gopath.Join(pathname, "d1")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)

	fn := gopath.Join(dn, "w")

	ch := make(chan error)
	done := make(chan bool)
	go func() {
		fsl, err := fslib.MakeFsLibAddr("fsl1", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(t, err)
		for i := 1; i < N; {
			_, err := fsl.PutFile(fn, 0777, sp.OWRITE, nil)
			assert.Equal(t, nil, err)
			err = ts.SetRemoveWatch(fn, func(fn string, r error) {
				// log.Printf("watch cb %v err %v\n", i, r)
				ch <- r
			})
			if err == nil {
				// log.Printf("wait for rm %v\n", i)
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
			time.Sleep(MS * time.Millisecond)
			ts.Remove(fn) // remove may fail
		}
	}

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

// Concurrently remove & watch, but watch may be set after remove.
func TestWatchRemoveConcurAsynchWatchSet(t *testing.T) {
	const N = 1000 // 10_000

	ts := test.MakeTstatePath(t, pathname)
	dn := gopath.Join(pathname, "d1")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)

	ch := make(chan error)
	done := make(chan bool)
	fsl, err := fslib.MakeFsLibAddr("fsl1", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
	assert.Nil(t, err)
	for i := 0; i < N; i++ {
		fn := gopath.Join(dn, strconv.Itoa(i))
		_, err := fsl.PutFile(fn, 0777, sp.OWRITE, nil)
		assert.Nil(t, err, "Err putfile: %v", err)
	}
	for i := 0; i < N; i++ {
		fn := gopath.Join(dn, strconv.Itoa(i))
		go func(fn string) {
			err := ts.SetRemoveWatch(fn, func(fn string, r error) {
				// log.Printf("watch cb %v err %v\n", i, r)
				ch <- r
			})
			// Either no error, or remove already happened.
			var serr *serr.Err
			assert.True(ts.T, err == nil || errors.As(err, &serr) && serr.IsErrNotfound(), "Unexpected RemoveWatch error: %v", err)
			done <- true
		}(fn)
		go func(fn string) {
			err := ts.Remove(fn)
			assert.Nil(t, err, "Unexpected remove error: %v", err)
		}(fn)
	}
	for i := 0; i < N; i++ {
		<-done
	}
	ts.Shutdown()
}

func TestConcurFile(t *testing.T) {
	const I = 20
	const N = 100
	ts := test.MakeTstatePath(t, pathname)
	ch := make(chan int)
	for i := 0; i < I; i++ {
		go func(i int) {
			for j := 0; j < N; j++ {
				fn := gopath.Join(pathname, "f"+strconv.Itoa(i))
				data := []byte(fn)
				_, err := ts.PutFile(fn, 0777, sp.OWRITE, data)
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
	for i := 0; i < I; i++ {
		<-ch
	}
	ts.Shutdown()
}

const (
	NFILE = 100 // 1000
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
			err = fsl.Rename(gopath.Join(TODO, st.Name), gopath.Join(DONE, st.Name+"."+t))
			if err == nil {
				i = i + 1
				ok = true
			} else {
				var serr *serr.Err
				assert.True(ts.T, errors.As(err, &serr) && serr.IsErrNotfound())
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
	ts := test.MakeTstatePath(t, pathname)
	cont := make(chan bool)
	done := make(chan int)
	TODO := gopath.Join(pathname, "todo")
	DONE := gopath.Join(pathname, "done")

	initfs(ts, TODO, DONE)

	// start N threads trying to rename files in todo dir
	for i := 0; i < N; i++ {
		fsl, err := fslib.MakeFsLibAddr("thread"+strconv.Itoa(i), sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
		assert.Nil(t, err)
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
		_, err := ts.PutFile(gopath.Join(TODO, "job"+strconv.Itoa(i)), 07000, sp.OWRITE, []byte{})
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

	err := ts.RmDir(TODO)
	assert.Nil(t, err, "RmDir: %v", err)
	err = ts.RmDir(DONE)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestSymlinkPath(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	dn := gopath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink([]byte(pathname), gopath.Join(pathname, "namedself"), 0777|sp.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir(gopath.Join(pathname, "namedself") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d", "namedself"}), "dir")

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func mkMount(t *testing.T, ts *test.Tstate, path string) sp.Tmount {
	mnt, left, err := ts.CopyMount(pathname)
	assert.Nil(t, err)
	mnt.SetTree(left)
	h, p, err := mnt.TargetHostPort()
	assert.Nil(t, err)
	if h == "" {
		a := net.JoinHostPort(ts.GetLocalIP(), p)
		mnt.SetAddr(sp.MkTaddrs([]string{a}))
	}
	return mnt
}

func TestMountSimple(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	dn := gopath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	pn := gopath.Join(pathname, "namedself")
	err = ts.MountService(pn, mkMount(t, ts, pathname))
	assert.Nil(ts.T, err, "MountService")
	sts, err := ts.GetDir(pn + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d", "namedself"}), "dir")

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)
	err = ts.Remove(pn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestUnionDir(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	dn := gopath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.MountService(gopath.Join(pathname, "d/namedself0"), mkMount(t, ts, pathname))
	assert.Nil(ts.T, err, "MountService")

	err = ts.MountService(gopath.Join(pathname, "d/namedself1"), sp.MkMountServer(":2222"))
	assert.Nil(ts.T, err, "MountService")

	sts, err := ts.GetDir(gopath.Join(pathname, "d/~any") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d"}), "dir")

	sts, err = ts.GetDir(gopath.Join(pathname, "d/~any/d") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"namedself0", "namedself1"}), "dir")

	sts, err = ts.GetDir(gopath.Join(pathname, "d/~local") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d"}), "dir")

	pn, err := ts.ResolveUnions(gopath.Join(pathname, "d/~local"))
	assert.Equal(t, nil, err)
	sts, err = ts.GetDir(pn)
	assert.Nil(t, err)
	assert.True(t, fslib.Present(sts, path.Path{"d"}), "dir")

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestUnionRoot(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	pn0 := gopath.Join(pathname, "namedself0")
	pn1 := gopath.Join(pathname, "namedself1")
	err := ts.MountService(pn0, mkMount(t, ts, pathname))
	assert.Nil(ts.T, err, "MountService")
	err = ts.MountService(pn1, sp.MkMountServer("xxx"))
	assert.Nil(ts.T, err, "MountService")

	sts, err := ts.GetDir(gopath.Join(pathname, "~any") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"namedself0", "namedself1"}), "dir")

	err = ts.Remove(pn0)
	assert.Nil(t, err)
	err = ts.Remove(pn1)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestUnionSymlinkRead(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	pn0 := gopath.Join(pathname, "namedself0")
	mnt := mkMount(t, ts, pathname)
	err := ts.MountService(pn0, mnt)
	assert.Nil(ts.T, err, "MountService")

	dn := gopath.Join(pathname, "d")
	err = ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.MountService(gopath.Join(pathname, "d/namedself1"), mnt)
	assert.Nil(ts.T, err, "MountService")

	sts, err := ts.GetDir(gopath.Join(pathname, "~any/d/namedself1") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d", "namedself0"}), "root wrong")

	sts, err = ts.GetDir(gopath.Join(pathname, "~any/d/namedself1/d") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"namedself1"}), "d wrong")

	err = ts.Remove(pn0)
	assert.Nil(t, err)

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestUnionSymlinkPut(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	pn0 := gopath.Join(pathname, "namedself0")
	err := ts.MountService(pn0, mkMount(t, ts, pathname))
	assert.Nil(ts.T, err, "MountService")

	b := []byte("hello")
	fn := gopath.Join(pathname, "~any/namedself0/f")
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, b)
	assert.Equal(t, nil, err)

	fn1 := gopath.Join(pathname, "~any/namedself0/g")
	_, err = ts.PutFile(fn1, 0777, sp.OWRITE, b)
	assert.Equal(t, nil, err)

	sts, err := ts.GetDir(gopath.Join(pathname, "~any/namedself0") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"f", "g"}), "root wrong")

	d, err := ts.GetFile(gopath.Join(pathname, "~any/namedself0/f"))
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, b, d, "GetFile")

	d, err = ts.GetFile(gopath.Join(pathname, "~any/namedself0/g"))
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, b, d, "GetFile")

	err = ts.Remove(pn0)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestSetFileSymlink(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.MountService(gopath.Join(pathname, "namedself0"), mkMount(t, ts, pathname))
	assert.Nil(ts.T, err, "MountService")

	st := stats.Stats{}
	err = ts.GetFileJson(gopath.Join("name", sp.STATSD), &st)
	assert.Nil(t, err, "statsd")
	nwalk := st.Nwalk

	d = []byte("byebye")
	n, err := ts.SetFile(gopath.Join(pathname, "namedself0/f"), d, sp.OWRITE, 0)
	assert.Nil(ts.T, err, "SetFile: %v", err)
	assert.Equal(ts.T, sessp.Tsize(len(d)), n, "SetFile")

	err = ts.GetFileJson(gopath.Join(pathname, sp.STATSD), &st)
	assert.Nil(t, err, "statsd")

	assert.NotEqual(ts.T, nwalk, st.Nwalk, "setfile")
	nwalk = st.Nwalk

	b, err := ts.GetFile(gopath.Join(pathname, "namedself0/f"))
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, d, b, "GetFile")

	err = ts.GetFileJson(gopath.Join(pathname, sp.STATSD), &st)
	assert.Nil(t, err, "statsd")

	assert.Equal(ts.T, nwalk, st.Nwalk, "getfile")

	err = ts.Remove(fn)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestMountUnion(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	dn := gopath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.MountService(gopath.Join(pathname, "d/namedself0"), sp.MkMountServer(":1111"))
	assert.Nil(ts.T, err, "MountService")

	pn := gopath.Join(pathname, "mount")
	err = ts.MountService(pn, mkMount(t, ts, dn))
	assert.Nil(ts.T, err, "MountService")

	sts, err := ts.GetDir(gopath.Join(pathname, "mount/~any") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d"}), "dir")

	err = ts.Remove(pn)
	assert.Nil(t, err, "Remove %v", err)
	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestOpenRemoveRead(t *testing.T) {
	ts := test.MakeTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
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
	ts := test.MakeTstatePath(t, pathname)

	dot := pathname + "/."

	// Make a new fsl for this test, because we want to use ts.FsLib
	// to shutdown the system.
	fsl, err := fslib.MakeFsLibAddr("fslibtest-1", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
	assert.Nil(t, err)

	// connect
	_, err = fsl.Stat(dot)
	assert.Nil(t, err)

	// close
	err = fsl.Exit()
	assert.Nil(t, err)

	_, err = fsl.Stat(dot)
	assert.NotNil(t, err)
	var serr *serr.Err
	assert.True(t, errors.As(err, &serr) && serr.IsErrUnreachable())

	ts.Shutdown()
}
