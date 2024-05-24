package fslib_test

import (
	"flag"
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
	"sigmaos/namesrv"
	"sigmaos/netproxyclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/~local/" or  "name/schedd/~local/"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func TestCompile(t *testing.T) {
}

func TestInitFs(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	sts, err := ts.GetDir(pathname)
	assert.Nil(t, err, "Error GetDir: %v", err)
	if pathname == sp.NAMED {
		db.DPrintf(db.TEST, "named %v %v\n", sp.Names(sts), sts[0])
		assert.True(t, fslib.Present(sts, namesrv.InitRootDir), "initfs")
		sts, err = ts.GetDir(pathname + "/boot")
		assert.Nil(t, err, "Err getdir: %v", err)
	} else {
		db.DPrintf(db.TEST, "%v %v\n", pathname, sp.Names(sts))
		assert.True(t, len(sts) >= 2, "initfs")
	}
	ts.Shutdown()
}

func TestEmptyPath(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	_, err := ts.GetDir(pathname)
	assert.Nil(t, err)
	_, err = ts.GetFile("")
	assert.NotNil(t, err)
	ts.Shutdown()
}

func TestRemoveBasic(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	b, err := ts.IsDir(dn)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, b)

	d := []byte("hello")
	_, err = ts.PutFile(filepath.Join(dn, "f"), 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	sts, err := ts.GetDir(dn)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(sts))
	assert.Equal(t, "f", sts[0].Name)
	qt := sts[0].Tqid().Ttype()
	assert.Equal(t, sp.QTFILE, qt)

	err = ts.RmDir(dn)
	_, err = ts.Stat(dn)
	assert.NotNil(t, err)

	ts.Shutdown()
}

func TestCreateTwice(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
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

func TestRemoveNonExistent(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
	d := []byte("hello")

	ts.Remove(fn) // remove from last test

	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove(filepath.Join(pathname, "this-file-does-not-exist"))
	assert.NotNil(t, err)

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestRemovePath(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	d1 := filepath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	fn := filepath.Join(d1, "f")
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	d1 := filepath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	from := filepath.Join(d1, "f")

	d := []byte("hello")
	_, err = ts.PutFile(from, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	to := filepath.Join(d1, "g")
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	d1 := filepath.Join(pathname, "d1")
	db.DPrintf(db.TEST, "path %v", pathname)
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	fn := filepath.Join(d1, "f")

	ep, err := ts.GetNamedEndpoint()
	assert.Nil(t, err, "GetNamedEndpoint: %v", err)
	err = ts.MkEndpointFile(fn, ep, sp.NoLeaseId)
	assert.Nil(t, err, "MkEndpointFile: %v", err)

	sts, err := ts.GetDir(fn + "/")
	assert.Nil(t, err, "GetDir: %v", err)
	assert.True(t, fslib.Present(sts, namesrv.InitRootDir))

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestRmDirWithSymlink(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	d1 := filepath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	fn := filepath.Join(d1, "f")

	ep, err := ts.GetNamedEndpoint()
	assert.Nil(t, err, "GetNamedEndpoint: %v", err)
	err = ts.MkEndpointFile(fn, ep, sp.NoLeaseId)
	assert.Nil(t, err, "MkEndpointFile: %v", err)

	sts, err := ts.GetDir(fn + "/")
	assert.Nil(t, err, "GetDir: %v", err)
	assert.True(t, fslib.Present(sts, namesrv.InitRootDir))

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestReadSymlink(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	d1 := filepath.Join(pathname, "d1")
	err := ts.MkDir(d1, 0777)
	assert.Nil(t, err, "Mkdir %v", err)
	fn := filepath.Join(d1, "f")

	ep, err := ts.GetNamedEndpoint()
	assert.Nil(t, err, "GetNamedEndpoint: %v", err)
	err = ts.MkEndpointFile(fn, ep, sp.NoLeaseId)
	assert.Nil(t, err, "MkEndpointFile: %v", err)

	_, err = ts.GetDir(fn + "/")
	assert.Nil(t, err, "GetDir: %v", err)

	ep1, err := ts.ReadEndpoint(fn)
	assert.Nil(t, err, "ReadEndpoint: %v", err)

	assert.Equal(t, ep.Addrs()[0].GetIP(), ep1.Addrs()[0].GetIP())
	assert.Equal(t, ep.Addrs()[0].GetPort(), ep1.Addrs()[0].GetPort())
	assert.Equal(t, ep.Addrs()[0].GetNetNS(), ep1.Addrs()[0].GetNetNS())

	err = ts.RmDir(d1)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestReadOff(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)

	rdr.Lseek(3)
	b := make([]byte, 10)
	n, err := rdr.Reader.Read(b)
	assert.Nil(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, "lo", string(b[:2]))

	fd, err := ts.Open(fn, sp.OREAD)
	assert.Nil(t, err)
	n1, err := ts.Pread(fd, b, 3)
	assert.Equal(t, sp.Tsize(2), n1)
	assert.Equal(t, "lo", string(b[:2]))

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestRenameAcrossDir(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	d1 := filepath.Join(pathname, "d1")
	d2 := filepath.Join(pathname, "d2")

	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir(d2, 0777)
	assert.Equal(t, nil, err)

	fn := filepath.Join(d1, "f")
	fn1 := filepath.Join(d2, "g")
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
	d1 := filepath.Join(pathname, "d1")
	d2 := filepath.Join(pathname, "d2")
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir(d2, 0777)
	assert.Equal(t, nil, err)

	fn := filepath.Join(d1, "f")
	fn1 := filepath.Join(d2, "g")
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
	d1 := filepath.Join(pathname, "d1")
	d2 := filepath.Join(pathname, "d2")

	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	err := ts.MkDir(d1, 0777)
	assert.Equal(t, nil, err)
	err = ts.MkDir(d2, 0777)
	assert.Equal(t, nil, err)

	fn := filepath.Join(d1, "f")
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	d := []byte("1234")
	fn := filepath.Join(pathname, "f")

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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	d := []byte("hello")
	src := filepath.Join(pathname, "f")
	dst := filepath.Join(pathname, "g")
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(pathname, "dir0")
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(pathname, "dir")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)
	// ts.SetChunkSz(sp.Tsize(512))
	n := 1000
	names := make([]string, 0)
	for i := 0; i < n; i++ {
		db.DPrintf(db.TEST, "Putfile %v", i)
		name := strconv.Itoa(i)
		names = append(names, name)
		_, err := ts.PutFile(filepath.Join(dn, name), 0777, sp.OWRITE, []byte(name))
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
	assert.Equal(t, n, i)

	db.DPrintf(db.TEST, "Pre RmDir")
	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func dirwriter(t *testing.T, pe *proc.ProcEnv, dn, name string, ch chan bool) {
	fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
	assert.Nil(t, err)
	stop := false
	for !stop {
		select {
		case stop = <-ch:
		default:
			err := fsl.Remove(filepath.Join(dn, name))
			assert.Nil(t, err, "Remove: %v", err)
			_, err = fsl.PutFile(filepath.Join(dn, name), 0777, sp.OWRITE|sp.OEXCL, []byte(name))
			assert.Nil(t, err, "Put: %v", err)
		}
	}
	err = fsl.Close()
	assert.Nil(t, err)
}

// Concurrently scan dir and create/remove entries
func TestDirConcur(t *testing.T) {
	const (
		N     = 1
		NFILE = 3
		NSCAN = 100
	)
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(pathname, "dir")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)

	for i := 0; i < NFILE; i++ {
		name := strconv.Itoa(i)
		_, err := ts.PutFile(filepath.Join(dn, name), 0777, sp.OWRITE, []byte(name))
		assert.Equal(t, nil, err)
	}

	ch := make(chan bool)
	for i := 0; i < N; i++ {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		go dirwriter(t, pe, dn, strconv.Itoa(i), ch)
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
	assert.Nil(t, err, "RmDir: %v %v", dn, err)

	ts.Shutdown()
}

func TestWaitCreate(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "w")
	ch := make(chan bool)

	go func() {
		db.DPrintf(db.TEST, "Invoke OpenWait")
		fd, err := ts.OpenWait(fn, sp.OREAD)
		assert.Nil(t, err)
		assert.NotEqual(t, -1, fd)
		db.DPrintf(db.TEST, "OpenWait done")
		ch <- true
	}()

	// give goroutine time to start
	time.Sleep(100 * time.Millisecond)

	db.DPrintf(db.TEST, "PutFile")

	_, err := ts.PutFile(fn, 0777, sp.OWRITE, nil)
	assert.Nil(t, err, "Error PutFile: %v", err)
	db.DPrintf(db.TEST, "PutFile done")

	db.DPrintf(db.TEST, "Wait for OpenWait to return")

	<-ch

	err = ts.Remove(fn)
	assert.Nil(t, err, "Remove: %v", err)

	ts.Shutdown()
}

func TestWaitRemoveOne(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "w")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, nil)
	assert.Equal(t, nil, err)

	ch := make(chan bool)
	go func() {
		err := ts.WaitRemove(fn)
		assert.Nil(t, err)
		ch <- true
	}()

	// give goroutine time to start
	time.Sleep(100 * time.Millisecond)

	db.DPrintf(db.TEST, "Remove %v", fn)

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	db.DPrintf(db.TEST, "Wait for RemoveWait to return")

	<-ch

	db.DPrintf(db.TEST, "RemoveWait returns")

	ts.Shutdown()
}

func TestWaitDir(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	f := "x"
	pn := filepath.Join(pathname, "d1")
	err := ts.MkDir(pn, 0777)
	assert.Equal(t, nil, err)
	ch := make(chan bool)
	go func() {
		err := ts.ReadDirWait(pn, func(sts []*sp.Stat) bool {
			db.DPrintf(db.TEST, "ReadDirWait %v\n", sp.Names(sts))
			for _, st := range sts {
				if st.Name == f {
					ch <- true
					return false
				}
			}
			return true
		})
		assert.Nil(t, err)
	}()

	// give goroutine time to start
	time.Sleep(100 * time.Millisecond)

	db.DPrintf(db.TEST, "Putfile")

	pn1 := filepath.Join(pn, f)
	_, err = ts.PutFile(pn1, 0777, sp.OWRITE, nil)
	assert.Equal(t, nil, err)

	db.DPrintf(db.TEST, "Putfile done %v", pn1)

	<-ch

	db.DPrintf(db.TEST, "ReadDirWait returned")

	err = ts.RmDir(pn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

// Concurrently remove & wait
func TestWaitRemoveWaitConcur(t *testing.T) {
	const N = 100 // 10_000

	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(pathname, "d1")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)

	done := make(chan bool)
	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
	assert.Nil(t, err)
	for i := 0; i < N; i++ {
		fn := filepath.Join(dn, strconv.Itoa(i))
		_, err := fsl.PutFile(fn, 0777, sp.OWRITE, nil)
		assert.Nil(t, err, "Err putfile: %v", err)
	}
	for i := 0; i < N; i++ {
		fn := filepath.Join(dn, strconv.Itoa(i))
		go func(fn string) {
			err := ts.WaitRemove(fn)
			assert.True(ts.T, err == nil, "Unexpected WaitRemove error: %v", err)
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
	err = fsl.Close()
	assert.Nil(t, err)
	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)
	ts.Shutdown()
}

// Concurrently wait, create and remove in dir
func TestWaitCreateRemoveConcur(t *testing.T) {
	const N = 500 // 5_000
	const MS = 2

	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dn := filepath.Join(pathname, "d1")
	err := ts.MkDir(dn, 0777)
	assert.Equal(t, nil, err)

	fn := filepath.Join(dn, "w")
	fn1 := filepath.Join(dn, "x")

	for i := 0; i < N; i++ {
		_, err = ts.PutFile(fn, 0777, sp.OWRITE, nil)
		assert.Equal(t, nil, err)
		done := make(chan bool)
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
		assert.Nil(t, err)

		go func() {
			err = fsl.WaitRemove(fn)
			if err == nil {
				// db.DPrintf(db.TEST, "wait for rm %v\n", i)
			} else {
				assert.True(t, serr.IsErrCode(err, serr.TErrNotfound))
				db.DPrintf(db.TEST, "WaitRemove %v err %v\n", fn, err)
			}
			done <- true
		}()
		time.Sleep(1 * time.Millisecond)
		_, err = ts.PutFile(fn1, 0777, sp.OWRITE, nil)
		assert.Equal(t, nil, err)
		time.Sleep(1 * time.Millisecond)
		err = ts.Remove(fn)
		assert.Equal(t, nil, err)
		<-done

		// cleanup
		err = ts.Remove(fn1)
		assert.Equal(t, nil, err)
		fsl.Close()
	}
	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestConcurFile(t *testing.T) {
	const I = 20
	const N = 100
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ch := make(chan int)
	for i := 0; i < I; i++ {
		go func(i int) {
			for j := 0; j < N; j++ {
				fn := filepath.Join(pathname, "f"+strconv.Itoa(i))
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
			err = fsl.Rename(filepath.Join(TODO, st.Name), filepath.Join(DONE, st.Name+"."+t))
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	cont := make(chan bool)
	done := make(chan int)
	TODO := filepath.Join(pathname, "todo")
	DONE := filepath.Join(pathname, "done")
	fsls := make([]*fslib.FsLib, 0, N)

	initfs(ts, TODO, DONE)

	// start N threads trying to rename files in todo dir
	for i := 0; i < N; i++ {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
		assert.Nil(t, err)
		fsls = append(fsls, fsl)
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
		_, err := ts.PutFile(filepath.Join(TODO, "job"+strconv.Itoa(i)), 07000, sp.OWRITE|sp.OEXCL, []byte{})
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

	for _, fsl := range fsls {
		err := fsl.Close()
		assert.Nil(t, err)
	}
	ts.Shutdown()
}

func TestConcurAssignedRename(t *testing.T) {
	const N = 20
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	cont := make(chan string)
	done := make(chan int)
	TODO := filepath.Join(pathname, "todo")
	DONE := filepath.Join(pathname, "done")
	fsls := make([]*fslib.FsLib, 0, N)
	initfs(ts, TODO, DONE)

	fnames := []string{}
	// generate files in the todo dir
	for i := 0; i < NFILE; i++ {
		fnames = append(fnames, "job"+strconv.Itoa(i))
		_, err := ts.PutFile(filepath.Join(TODO, fnames[i]), 07000, sp.OWRITE, []byte{})
		assert.Nil(ts.T, err, "Create job")
	}

	// start N threads trying to rename files in todo dir
	for i := 0; i < N; i++ {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
		assert.Nil(t, err, "Err newfslib: %v", err)
		fsls = append(fsls, fsl)
		go func(fsl *fslib.FsLib, t string) {
			n := 0
			for {
				fname := <-cont
				if fname == "STOP" {
					done <- n
					return
				}
				err := fsl.Rename(filepath.Join(TODO, fname), filepath.Join(DONE, fname))
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

	for _, fsl := range fsls {
		err := fsl.Close()
		assert.Nil(t, err)
	}
	ts.Shutdown()
}

func TestSymlinkPath(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	dn := filepath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink([]byte(pathname), filepath.Join(pathname, "namedself"), 0777)
	assert.Nil(ts.T, err, "Symlink")

	sts, err := ts.GetDir(filepath.Join(pathname, "namedself") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d", "namedself"}), "dir")

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func newEndpoint(t *testing.T, ts *test.Tstate, path string) *sp.Tendpoint {
	ep, left, err := ts.CopyEndpoint(pathname)
	db.DPrintf(db.ALWAYS, "copy ep %v", ep)
	assert.Nil(t, err)
	ep.SetTree(left)
	h, p := ep.TargetIPPort(0)
	if h == "" {
		ts.SetLocalMount(ep, p)
	}
	return ep
}

func TestEndpointSimple(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	dn := filepath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	pn := filepath.Join(pathname, "namedself")
	err = ts.MkEndpointFile(pn, newEndpoint(t, ts, pathname), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	dn := filepath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.MkEndpointFile(filepath.Join(pathname, "d/namedself0"), newEndpoint(t, ts, pathname), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")

	newep := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, 2222, ts.ProcEnv().GetNet())}, sp.ROOTREALM)
	err = ts.MkEndpointFile(filepath.Join(pathname, "d/namedself1"), newep, sp.NoLeaseId)
	assert.Nil(ts.T, err, "EndpointService")

	sts, err := ts.GetDir(filepath.Join(pathname, "d/~any") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d"}), "dir")

	sts, err = ts.GetDir(filepath.Join(pathname, "d/~any/d") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"namedself0", "namedself1"}), "dir")

	sts, err = ts.GetDir(filepath.Join(pathname, "d/~local") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d"}), "dir")

	pn, err := ts.ResolveMounts(filepath.Join(pathname, "d/~local"))
	assert.Equal(t, nil, err)
	sts, err = ts.GetDir(pn)
	assert.Nil(t, err)
	assert.True(t, fslib.Present(sts, path.Path{"d"}), "dir")

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestUnionRoot(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pn0 := filepath.Join(pathname, "namedself0")
	pn1 := filepath.Join(pathname, "namedself1")
	err := ts.MkEndpointFile(pn0, newEndpoint(t, ts, pathname), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")
	newep := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddr("xxx", sp.INNER_CONTAINER_IP, sp.NO_PORT)}, sp.ROOTREALM)
	err = ts.MkEndpointFile(pn1, newep, sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")

	pn := pathname
	if pathname != sp.NAMED && pathname != "name/memfs/~local/" {
		pn = filepath.Join(pathname, "~any")
	}
	sts, err := ts.GetDir(pn + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"namedself0", "namedself1"}), "dir: %v", sp.Names(sts))

	err = ts.Remove(pn0)
	assert.Nil(t, err)
	err = ts.Remove(pn1)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestUnionSymlinkRead(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pn0 := filepath.Join(pathname, "namedself0")
	ep := newEndpoint(t, ts, pathname)
	err := ts.MkEndpointFile(pn0, ep, sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")

	dn := filepath.Join(pathname, "d")
	err = ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.MkEndpointFile(filepath.Join(pathname, "d/namedself1"), ep, sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")

	basepn := pathname
	if pathname != sp.NAMED && pathname != "name/memfs/~local/" {
		basepn = filepath.Join(pathname, "~any")
	}
	sts, err := ts.GetDir(filepath.Join(basepn, "d/namedself1") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d", "namedself0"}), "root wrong")

	sts, err = ts.GetDir(filepath.Join(basepn, "d/namedself1/d") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"namedself1"}), "d wrong")

	err = ts.Remove(pn0)
	assert.Nil(t, err)

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestUnionSymlinkPut(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pn0 := filepath.Join(pathname, "namedself0")
	err := ts.MkEndpointFile(pn0, newEndpoint(t, ts, pathname), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")

	b := []byte("hello")
	basepn := pathname
	if pathname != sp.NAMED && pathname != "name/memfs/~local/" {
		basepn = filepath.Join(pathname, "~any")
	}
	fn := filepath.Join(basepn, "namedself0/f")
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, b)
	assert.Equal(t, nil, err)

	fn1 := filepath.Join(basepn, "namedself0/g")
	_, err = ts.PutFile(fn1, 0777, sp.OWRITE, b)
	assert.Equal(t, nil, err)

	sts, err := ts.GetDir(filepath.Join(basepn, "namedself0") + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"f", "g"}), "root wrong")

	d, err := ts.GetFile(filepath.Join(basepn, "namedself0/f"))
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, b, d, "GetFile")

	d, err = ts.GetFile(filepath.Join(basepn, "namedself0/g"))
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, b, d, "GetFile")

	err = ts.Remove(pn0)
	assert.Nil(t, err)

	err = ts.Remove(filepath.Join(pathname, "g"))
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestSetFileSymlink(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
	d := []byte("hello")
	_, err := ts.PutFile(fn, 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.MkEndpointFile(filepath.Join(pathname, "namedself0"), newEndpoint(t, ts, pathname), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")

	st, err := ts.ReadStats(pathname)
	assert.Nil(t, err, "statsd")
	nwalk := st.Counters["Nwalk"]

	db.DPrintf(db.TEST, "st %v\n", st)

	d = []byte("byebye")
	n, err := ts.SetFile(filepath.Join(pathname, "namedself0/f"), d, sp.OWRITE, 0)
	assert.Nil(ts.T, err, "SetFile: %v", err)
	assert.Equal(ts.T, sp.Tsize(len(d)), n, "SetFile")

	st, err = ts.ReadStats(pathname)
	assert.Nil(t, err, "statsd")

	db.DPrintf(db.TEST, "st %v\n", st)

	assert.NotEqual(ts.T, nwalk, st.Counters["Nwalk"], "setfile")
	nwalk = st.Counters["Nwalk"]

	b, err := ts.GetFile(filepath.Join(pathname, "namedself0/f"))
	assert.Nil(ts.T, err, "GetFile")
	assert.Equal(ts.T, d, b, "GetFile")

	st, err = ts.ReadStats(pathname)
	assert.Nil(t, err, "statsd")

	assert.Equal(ts.T, nwalk, st.Counters["Nwalk"], "getfile")

	err = ts.Remove(fn)
	assert.Nil(t, err)

	err = ts.Remove(filepath.Join(pathname, "namedself0"))
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestEndpointUnion(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	dn := filepath.Join(pathname, "d")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	newep := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, 1111, ts.ProcEnv().GetNet())}, sp.ROOTREALM)
	err = ts.MkEndpointFile(filepath.Join(pathname, "d/namedself0"), newep, sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")

	pn := filepath.Join(pathname, "mount")
	err = ts.MkEndpointFile(pn, newEndpoint(t, ts, dn), sp.NoLeaseId)
	assert.Nil(ts.T, err, "MkEndpointFile")

	eppn := "mount/"
	if pathname != sp.NAMED && pathname != "name/memfs/~local/" {
		eppn = filepath.Join(eppn, "~any")
	}

	sts, err := ts.GetDir(filepath.Join(pathname, eppn) + "/")
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, path.Path{"d"}), "dir")

	err = ts.Remove(pn)
	assert.Nil(t, err, "Remove %v", err)
	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestOpenRemoveRead(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
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

func TestFslibClose(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	dot := pathname + "/."

	// Make a new fsl for this test, because we want to use ts.FsLib
	// to shutdown the system.
	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
	assert.Nil(t, err)

	// connect
	_, err = fsl.Stat(dot)
	assert.Nil(t, err)

	// Detach from servers
	err = fsl.Close()
	assert.Nil(t, err)

	_, err = fsl.Stat(dot)
	assert.NotNil(t, err)
	assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))

	ts.Shutdown()
}

func TestEphemeralFileOK(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")

	li, err := ts.LeaseClnt.AskLease(fn, fsetcd.LeaseTTL)
	assert.Nil(t, err, "Error AskLease: %v", err)

	li.KeepExtending()

	_, err = ts.PutFileEphemeral(fn, 0777, sp.OWRITE, li.Lease(), nil)
	assert.Nil(t, err)

	time.Sleep(fsetcd.LeaseTTL*time.Second + 1)

	_, err = ts.Stat(fn)
	assert.Nil(t, err)

	err = ts.Remove(fn)
	assert.Nil(t, err)

	ts.LeaseClnt.EndLeases()

	ts.Shutdown()
}

func TestEphemeralFileExpire(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	dn := filepath.Join(pathname, "dir")
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	fn := filepath.Join(dn, "foobar")

	li, err := ts.LeaseClnt.AskLease(fn, fsetcd.LeaseTTL)
	assert.Nil(t, err, "Error AskLease: %v", err)

	_, err = ts.PutFileEphemeral(fn, 0777, sp.OWRITE, li.Lease(), nil)
	assert.Nil(t, err, "Err PutEphemeral: %v", err)

	sts, err := ts.GetDir(dn)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(sts))

	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)

	_, err = ts.Stat(fn)
	assert.NotNil(t, err, fn)

	sts, err = ts.GetDir(dn)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(sts))

	db.DPrintf(db.TEST, "names %v", sp.Names(sts))

	err = ts.RmDir(dn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestDisconnect(t *testing.T) {
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join(pathname, "f")
	d := []byte("hello")
	fd, err := ts.Create(fn, 0777, sp.OWRITE)
	assert.Equal(t, nil, err)
	_, err = ts.Write(fd, d)
	assert.Equal(t, nil, err)

	err = ts.Disconnect(fn)
	assert.Nil(t, err, "Disconnect")
	time.Sleep(100 * time.Millisecond)

	_, err = ts.Write(fd, d)
	assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))

	err = ts.CloseFd(fd)
	assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))

	fd, err = ts.Open(fn, sp.OREAD)
	assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable), "Err not unreachable: %v", err)

	ts.Shutdown()
}
