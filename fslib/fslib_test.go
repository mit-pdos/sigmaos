package fslib_test

import (
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
	"sigmaos/fsetcd"
	"sigmaos/fslib"
	"sigmaos/named"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/stats"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/~local/" or  "name/schedd/~local/"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func TestInitFs(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)
	sts, err := ts.GetDir(pathname)
	assert.Nil(t, err)
	if pathname == sp.NAMED {
		log.Printf("named %v\n", sp.Names(sts))
		assert.True(t, fslib.Present(sts, named.InitRootDir), "initfs")
		sts, err = ts.GetDir(pathname + "/boot")
		assert.Nil(t, err)
	} else {
		log.Printf("%v %v\n", pathname, sp.Names(sts))
		assert.True(t, len(sts) >= 2, "initfs")
	}
	ts.Shutdown()
}

func TestRemoveBasic(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	db.DPrintf(db.TEST, "PutFile")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "PutFile done")

	db.DPrintf(db.TEST, "RemoveFile")
	err = ts.Remove(fn)
	assert.Equal(t, nil, err)
	db.DPrintf(db.TEST, "RemoveFile done")

	db.DPrintf(db.TEST, "StatFile")
	_, err = ts.Stat(fn)
	assert.NotEqual(t, nil, err)
	db.DPrintf(db.TEST, "StatFile done")

	ts.Shutdown()
}

func TestDirBasic(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)
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
	qt := sts[0].Qid.Ttype()
	assert.Equal(t, sp.QTFILE, qt)

	err = ts.RmDir(dn)
	_, err = ts.Stat(dn)
	assert.NotNil(t, err)

	ts.Shutdown()
}

func TestCreateTwice(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Nil(t, err)
	_, err = ts.PutFile(fn, 0777, sp.OWRITE|sp.OEXCL, d)
	assert.NotNil(t, err)
	assert.True(t, serr.IsErrCode(err, serr.TErrExists))

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestConnect(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	fd, err := ts.Create(fn, 0777, sp.OWRITE)
	assert.Equal(t, nil, err)
	_, err = ts.Write(fd, d)
	assert.Equal(t, nil, err)

	srv, _, err := ts.PathLastSymlink(pathname)
	assert.Nil(t, err)

	err = ts.Disconnect(srv.String())
	assert.Nil(t, err, "Disconnect")
	time.Sleep(100 * time.Millisecond)
	db.DPrintf(db.TEST, "disconnected")

	_, err = ts.Write(fd, d)
	assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))

	err = ts.Close(fd)
	assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))

	fd, err = ts.Open(fn, sp.OREAD)
	assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable), "Err not unreachable: %v", err)

	ts.Shutdown()
}

func TestRemoveNonExistent(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")

	ts.Remove(fn) // remove from last test

	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove(gopath.Join(pathname, "this-file-does-not-exist"))
	assert.NotNil(t, err)

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestRemovePath(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

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
	ts := test.NewTstatePath(t, pathname)

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
	ts := test.NewTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	db.DPrintf(db.TEST, "path %v", pathname)
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	fn := gopath.Join(d1, "f")

	mnt := ts.GetNamedMount()
	err = ts.NewMountSymlink(fn, mnt, sp.NoLeaseId)
	assert.Nil(t, err, "NewMount: %v", err)

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
	ts := test.NewTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	fn := gopath.Join(d1, "f")

	mnt := ts.GetNamedMount()
	err = ts.NewMountSymlink(fn, mnt, sp.NoLeaseId)
	assert.Nil(t, err, "NewMount: %v", err)

	sts, err := ts.GetDir(fn + "/")
	assert.Nil(t, err, "GetDir: %v", err)
	assert.True(t, fslib.Present(sts, named.InitRootDir))

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestReadSymlink(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	d1 := gopath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	fn := gopath.Join(d1, "f")

	mnt := ts.GetNamedMount()
	err = ts.NewMountSymlink(fn, mnt, sp.NoLeaseId)
	assert.Nil(t, err, "NewMount: %v", err)

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
	ts := test.NewTstatePath(t, pathname)

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
	ts := test.NewTstatePath(t, pathname)

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
	ts := test.NewTstatePath(t, pathname)
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

	ts := test.NewTstatePath(t, pathname)
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
	ts := test.NewTstatePath(t, pathname)
	d := []byte("1234")
	fn := gopath.Join(pathname, "f")

	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)
	l, err := ts.SetFile(fn, d, sp.OAPPEND, sp.NoOffset)
	assert.Equal(t, nil, err)
	assert.Equal(t, sp.Tsize(len(d)), l)
	b, err := ts.GetFile(fn)
	assert.Equal(t, nil, err)
	assert.Equal(t, len(d)*2, len(b))

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestCopy(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)
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
	ts := test.NewTstatePath(t, pathname)
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
	ts := test.NewTstatePath(t, pathname)
	dn := gopath.Join(pathname, "dir")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	ts.SetChunkSz(sp.Tsize(512))
	n := 1000
	names := make([]string, 0)
	for i := 0; i < n; i++ {
		db.DPrintf(db.TEST, "Putfile %v", i)
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
		db.DPrintf(db.TEST, "ProcessDir %v", i)
		assert.Equal(t, names[i], st.Name)
		i += 1
		return false, nil

	})
	assert.Equal(t, i, n)

	db.DPrintf(db.TEST, "Pre RmDir")
	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func dirwriter(t *testing.T, pcfg *proc.ProcEnv, dn, name string, ch chan bool) {
	fsl, err := fslib.NewFsLib(pcfg)
	assert.Nil(t, err)
	stop := false
	for !stop {
		select {
		case stop = <-ch:
		default:
			err := fsl.Remove(gopath.Join(dn, name))
			assert.Nil(t, err, "Remove: %v", err)
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
	ts := test.NewTstatePath(t, pathname)
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
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
		go dirwriter(t, pcfg, dn, strconv.Itoa(i), ch)
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

func TestWatchCreate(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	fn := gopath.Join(pathname, "w")
	ch := make(chan bool)
	fd, err := ts.OpenWatch(fn, sp.OREAD, func(string, error) {
		ch <- true
		db.DPrintf(db.TEST, "Watch done")
	})
	assert.NotNil(t, err, "Err not nil: %v", err)
	assert.Equal(t, -1, fd, err)

	assert.True(t, serr.IsErrCode(err, serr.TErrNotfound))

	// give Watch goroutine to start
	time.Sleep(100 * time.Millisecond)

	db.DPrintf(db.TEST, "PutFile")
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, nil)
	assert.Nil(t, err, "Error PutFile: %v", err)
	db.DPrintf(db.TEST, "PutFile done")

	<-ch

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestWatchRemoveOne(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

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
	ts := test.NewTstatePath(t, pathname)

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

//func TestWatchRemoveConcur(t *testing.T) {
//	const N = 50 // 5_000
//	const MS = 10
//
//	ts := test.NewTstatePath(t, pathname)
//	dn := gopath.Join(pathname, "d1")
//	err := ts.MkDir(dn, 0777)
//	assert.Equal(t, nil, err)
//
//	fn := gopath.Join(dn, "w")
//
//	ch := make(chan error)
//	done := make(chan bool)
//	go func() {
//		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
//		fsl, err := fslib.NewFsLib(pcfg)
//		assert.Nil(t, err)
//		for i := 1; i < N; {
//			db.DPrintf(db.TEST, "PutFile %v", i)
//			_, err := fsl.PutFile(fn, 0777, sp.OWRITE, nil)
//			assert.Equal(t, nil, err)
//			err = ts.SetRemoveWatch(fn, func(fn string, r error) {
//				// log.Printf("watch cb %v err %v\n", i, r)
//				ch <- r
//			})
//			if err == nil {
//				// log.Printf("wait for rm %v\n", i)
//				r := <-ch
//				if r == nil {
//					i += 1
//				}
//			} else {
//				db.DPrintf(db.TEST, "SetRemoveWatch %v err %v\n", i, err)
//				// log.Printf("SetRemoveWatch %v err %v\n", i, err)
//			}
//		}
//		done <- true
//	}()
//
//	stop := false
//	for !stop {
//		select {
//		case <-done:
//			stop = true
//			db.DPrintf(db.TEST, "Done")
//		default:
//			time.Sleep(MS * time.Millisecond)
//			ts.Remove(fn) // remove may fail
//			db.DPrintf(db.TEST, "RemoveFile")
//		}
//	}
//
//	err = ts.RmDir(dn)
//	assert.Nil(t, err, "RmDir: %v", err)
//
//	ts.Shutdown()
//}

// Concurrently remove & watch, but watch may be set after remove.
func TestWatchRemoveConcurAsynchWatchSet(t *testing.T) {
	const N = 100 // 10_000

	ts := test.NewTstatePath(t, pathname)
	dn := gopath.Join(pathname, "d1")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)

	ch := make(chan error)
	done := make(chan bool)
	pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
	fsl, err := fslib.NewFsLib(pcfg)
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
			assert.True(ts.T, err == nil || serr.IsErrCode(err, serr.TErrNotfound), "Unexpected RemoveWatch error: %v", err)
			done <- true
		}(fn)
		go func(fn string) {
			err := ts.Remove(fn)
			assert.Nil(t, err, "Unexpected remove error: %v", err)
			done <- true
		}(fn)
	}
	for i := 0; i < 2*N; i++ {
		<-done
	}

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)
	ts.Shutdown()
}

func TestConcurFile(t *testing.T) {
	const I = 20
	const N = 100
	ts := test.NewTstatePath(t, pathname)
	ch := make(chan int)
	for i := 0; i < I; i++ {
		go func(i int) {
			for j := 0; j < N; j++ {
				fn := gopath.Join(pathname, "f"+strconv.Itoa(i))
				data := []byte(fn)
				_, err := ts.PutFile(fn, 0777, sp.OWRITE, data)
				assert.Nil(t, err, "Err PutFile: %v", err)
				d, err := ts.GetFile(fn)
				assert.Nil(t, err, "Err GetFile: %v", err)
				assert.Equal(t, len(data), len(d))
				err = ts.Remove(fn)
				assert.Nil(t, err, "Err Remove: %v", err)
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
	NFILE = 200 //1000
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
				assert.True(ts.T, serr.IsErrCode(err, serr.TErrNotfound))
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
	ts := test.NewTstatePath(t, pathname)
	cont := make(chan bool)
	done := make(chan int)
	TODO := gopath.Join(pathname, "todo")
	DONE := gopath.Join(pathname, "done")

	initfs(ts, TODO, DONE)

	// start N threads trying to rename files in todo dir
	for i := 0; i < N; i++ {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
		fsl, err := fslib.NewFsLib(pcfg)
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

func TestConcurAssignedRename(t *testing.T) {
	const N = 20
	ts := test.NewTstatePath(t, pathname)
	cont := make(chan string)
	done := make(chan int)
	TODO := gopath.Join(pathname, "todo")
	DONE := gopath.Join(pathname, "done")

	initfs(ts, TODO, DONE)

	fnames := []string{}
	// generate files in the todo dir
	for i := 0; i < NFILE; i++ {
		fnames = append(fnames, "job"+strconv.Itoa(i))
		_, err := ts.PutFile(gopath.Join(TODO, fnames[i]), 07000, sp.OWRITE, []byte{})
		assert.Nil(ts.T, err, "Create job")
	}

	// start N threads trying to rename files in todo dir
	for i := 0; i < N; i++ {
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
		fsl, err := fslib.NewFsLib(pcfg)
		assert.Nil(t, err, "Err newfslib: %v", err)
		go func(fsl *fslib.FsLib, t string) {
			n := 0
			for {
				fname := <-cont
				if fname == "STOP" {
					done <- n
					return
				}
				err := fsl.Rename(gopath.Join(TODO, fname), gopath.Join(DONE, fname))
				assert.Nil(ts.T, err, "Error rename: %v", err)
				n++
			}
		}(fsl, strconv.Itoa(i))
	}

	// Assign renames to goroutines
	for _, fn := range fnames {
		cont <- fn
	}

	// tell threads we are done with generating files
	n := 0
	for i := 0; i < N; i++ {
		cont <- "STOP"
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
	ts := test.NewTstatePath(t, pathname)

	dn := gopath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink([]byte(pathname), gopath.Join(pathname, "namedself"), 0777)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir(gopath.Join(pathname, "namedself") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d", "namedself"}), "dir")

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func newMount(t *testing.T, ts *test.Tstate, path string) sp.Tmount {
	mnt, left, err := ts.CopyMount(pathname)
	assert.Nil(t, err)
	mnt.SetTree(left)
	h, p, err := mnt.TargetHostPort()
	assert.Nil(t, err)
	if h == "" {
		a := net.JoinHostPort(ts.GetLocalIP(), p)
		mnt.SetAddr(sp.NewTaddrs([]string{a}))
	}
	return mnt
}

func TestMountSimple(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	dn := gopath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	pn := gopath.Join(pathname, "namedself")
	err = ts.MountService(pn, newMount(t, ts, pathname), sp.NoLeaseId)
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
	ts := test.NewTstatePath(t, pathname)

	dn := gopath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.MountService(gopath.Join(pathname, "d/namedself0"), newMount(t, ts, pathname), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MountService")

	err = ts.MountService(gopath.Join(pathname, "d/namedself1"), sp.NewMountServer(":2222"), sp.NoLeaseId)
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
	ts := test.NewTstatePath(t, pathname)

	pn0 := gopath.Join(pathname, "namedself0")
	pn1 := gopath.Join(pathname, "namedself1")
	err := ts.MountService(pn0, newMount(t, ts, pathname), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MountService")
	err = ts.MountService(pn1, sp.NewMountServer("xxx"), sp.NoLeaseId)
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
	ts := test.NewTstatePath(t, pathname)

	pn0 := gopath.Join(pathname, "namedself0")
	mnt := newMount(t, ts, pathname)
	err := ts.MountService(pn0, mnt, sp.NoLeaseId)
	assert.Nil(ts.T, err, "MountService")

	dn := gopath.Join(pathname, "d")
	err = ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.MountService(gopath.Join(pathname, "d/namedself1"), mnt, sp.NoLeaseId)
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
	ts := test.NewTstatePath(t, pathname)

	pn0 := gopath.Join(pathname, "namedself0")
	err := ts.MountService(pn0, newMount(t, ts, pathname), sp.NoLeaseId)
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

	err = ts.Remove(gopath.Join(pathname, "g"))
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestSetFileSymlink(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.MountService(gopath.Join(pathname, "namedself0"), newMount(t, ts, pathname), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MountService")

	st := stats.Stats{}
	err = ts.GetFileJson(gopath.Join("name", sp.STATSD), &st)
	assert.Nil(t, err, "statsd")
	nwalk := st.Nwalk

	d = []byte("byebye")
	n, err := ts.SetFile(gopath.Join(pathname, "namedself0/f"), d, sp.OWRITE, 0)
	assert.Nil(ts.T, err, "SetFile: %v", err)
	assert.Equal(ts.T, sp.Tsize(len(d)), n, "SetFile")

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

	err = ts.Remove(gopath.Join(pathname, "namedself0"))
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestMountUnion(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	dn := gopath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.MountService(gopath.Join(pathname, "d/namedself0"), sp.NewMountServer(":1111"), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MountService")

	pn := gopath.Join(pathname, "mount")
	err = ts.MountService(pn, newMount(t, ts, dn), sp.NoLeaseId)
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
	ts := test.NewTstatePath(t, pathname)

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

func TestFslibDetach(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	dot := pathname + "/."

	// Make a new fsl for this test, because we want to use ts.FsLib
	// to shutdown the system.
	pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), 1)
	fsl, err := fslib.NewFsLib(pcfg)
	assert.Nil(t, err)

	// connect
	_, err = fsl.Stat(dot)
	assert.Nil(t, err)

	// close
	err = fsl.DetachAll()
	assert.Nil(t, err)

	_, err = fsl.Stat(dot)
	assert.NotNil(t, err)
	assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))

	ts.Shutdown()
}

func TestEphemeralFileOK(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	fn := gopath.Join(pathname, "f")

	li, err := ts.LeaseClnt.AskLease(fn, fsetcd.LeaseTTL)
	assert.Nil(t, err)

	li.KeepExtending()

	_, err = ts.PutFileEphemeral(fn, 0777, sp.OWRITE, li.Lease(), nil)
	assert.Nil(t, err)

	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	_, err = ts.Stat(fn)
	assert.Nil(t, err)

	err = ts.Remove(fn)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestEphemeralFileExpire(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)

	fn := gopath.Join(pathname, "foobar")

	li, err := ts.LeaseClnt.AskLease(fn, fsetcd.LeaseTTL)
	assert.Nil(t, err)

	_, err = ts.PutFileEphemeral(fn, 0777, sp.OWRITE, li.Lease(), nil)
	assert.Nil(t, err)

	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	_, err = ts.Stat(fn)
	assert.NotNil(t, err)

	ts.Shutdown()
}
