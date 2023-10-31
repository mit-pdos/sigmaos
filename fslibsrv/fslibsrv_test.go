package fslibsrv_test

import (
	"bufio"
	"flag"
	"fmt"
	gopath "path"
	"strconv"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslib"
	"sigmaos/path"
	"sigmaos/perf"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/~local/"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

const (
	KBYTE = 1 << 10
	NRUNS = 3
	// SYNCFILESZ = 100 * KBYTE
	SYNCFILESZ = WRITESZ
	// FILESZ     = 100 * sp.MBYTE
	FILESZ  = SYNCFILESZ
	WRITESZ = 4096
)

func measure(p *perf.Perf, msg string, f func() sp.Tlength) sp.Tlength {
	totStart := time.Now()
	tot := sp.Tlength(0)
	for i := 0; i < NRUNS; i++ {
		start := time.Now()
		sz := f()
		tot += sz
		p.TptTick(float64(sz))
		ms := time.Since(start).Milliseconds()
		db.DPrintf(db.TEST, "%v: %s took %vms (%s)", msg, humanize.Bytes(uint64(sz)), ms, test.TputStr(sz, ms))
	}
	ms := time.Since(totStart).Milliseconds()
	db.DPrintf(db.ALWAYS, "Average %v: %s took %vms (%s)", msg, humanize.Bytes(uint64(tot)), ms, test.TputStr(tot, ms))
	return tot
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
	db.DPrintf(db.TEST, "%v: %d entries took %vms (%.1f file/s)", msg, n, tot, float64(n)/s)
}

type Thow uint8

const (
	HSYNC Thow = iota + 1
	HBUF
	HASYNC
)

func newFile(t *testing.T, fsl *fslib.FsLib, fn string, how Thow, buf []byte, sz sp.Tlength) sp.Tlength {
	switch how {
	case HSYNC:
		w, err := fsl.CreateWriter(fn, 0777, sp.OWRITE)
		assert.Nil(t, err, "Error Create writer: %v", err)
		err = test.Writer(t, w, buf, sz)
		assert.Nil(t, err)
		err = w.Close()
		assert.Nil(t, err)
	case HBUF:
		w, err := fsl.CreateWriter(fn, 0777, sp.OWRITE)
		assert.Nil(t, err, "Error Create writer: %v", err)
		bw := bufio.NewWriterSize(w, sp.BUFSZ)
		err = test.Writer(t, bw, buf, sz)
		assert.Nil(t, err)
		err = bw.Flush()
		assert.Nil(t, err)
		err = w.Close()
		assert.Nil(t, err)
	case HASYNC:
		w, err := fsl.CreateAsyncWriter(fn, 0777, sp.OWRITE)
		assert.Nil(t, err, "Error Create writer: %v", err)
		err = test.Writer(t, w, buf, sz)
		assert.Nil(t, err)
		err = w.Close()
		assert.Nil(t, err)
	}
	st, err := fsl.Stat(fn)
	assert.Nil(t, err)
	assert.Equal(t, sp.Tlength(sz), st.Tlength(), "stat")
	return sz
}

func TestWriteFilePerfSingle(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)
	fn := gopath.Join(pathname, "f")
	buf := test.NewBuf(WRITESZ)
	// Remove just in case it was left over from a previous run.
	ts.Remove(fn)
	p1, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.WRITER.String())
	assert.Nil(t, err)
	defer p1.Done()
	measure(p1, "writer", func() sp.Tlength {
		sz := newFile(t, ts.FsLib, fn, HSYNC, buf, SYNCFILESZ)
		err := ts.Remove(fn)
		assert.Nil(t, err)
		return sz
	})
	p2, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.BUFWRITER)
	assert.Nil(t, err)
	defer p2.Done()
	measure(p2, "bufwriter", func() sp.Tlength {
		sz := newFile(t, ts.FsLib, fn, HBUF, buf, FILESZ)
		err := ts.Remove(fn)
		assert.Nil(t, err)
		return sz
	})
	p3, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.ABUFWRITER)
	assert.Nil(t, err)
	defer p3.Done()
	measure(p3, "abufwriter", func() sp.Tlength {
		sz := newFile(t, ts.FsLib, fn, HASYNC, buf, FILESZ)
		err := ts.Remove(fn)
		assert.Nil(t, err)
		return sz
	})
	ts.Shutdown()
}

func TestWriteFilePerfMultiClient(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)
	N_CLI := 10
	buf := test.NewBuf(WRITESZ)
	done := make(chan sp.Tlength)
	fns := make([]string, 0, N_CLI)
	fsls := make([]*fslib.FsLib, 0, N_CLI)
	for i := 0; i < N_CLI; i++ {
		fns = append(fns, gopath.Join(pathname, "f"+strconv.Itoa(i)))
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
		fsl, err := fslib.NewFsLib(pcfg)
		assert.Nil(t, err)
		fsls = append(fsls, fsl)
	}
	// Remove just in case it was left over from a previous run.
	for _, fn := range fns {
		ts.Remove(fn)
	}
	p1, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.WRITER.String())
	assert.Nil(t, err)
	defer p1.Done()
	start := time.Now()
	for i := range fns {
		go func(i int) {
			n := measure(p1, "writer", func() sp.Tlength {
				sz := newFile(t, fsls[i], fns[i], HSYNC, buf, SYNCFILESZ)
				err := ts.Remove(fns[i])
				assert.Nil(t, err, "Remove err %v", err)
				return sz
			})
			done <- n
		}(i)
	}
	n := sp.Tlength(0)
	for _ = range fns {
		n += <-done
	}
	ms := time.Since(start).Milliseconds()
	db.DPrintf(db.ALWAYS, "Total tpt writer: %s took %vms (%s)", humanize.Bytes(uint64(n)), ms, test.TputStr(n, ms))
	p2, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.BUFWRITER)
	assert.Nil(t, err)
	defer p2.Done()
	start = time.Now()
	for i := range fns {
		go func(i int) {
			n := measure(p2, "bufwriter", func() sp.Tlength {
				sz := newFile(t, fsls[i], fns[i], HBUF, buf, FILESZ)
				err := ts.Remove(fns[i])
				assert.Nil(t, err, "Remove err %v", err)
				return sz
			})
			done <- n
		}(i)
	}
	n = 0
	for _ = range fns {
		n += <-done
	}
	ms = time.Since(start).Milliseconds()
	db.DPrintf(db.ALWAYS, "Total tpt bufwriter: %s took %vms (%s)", humanize.Bytes(uint64(n)), ms, test.TputStr(n, ms))
	p3, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.ABUFWRITER)
	assert.Nil(t, err)
	defer p3.Done()
	start = time.Now()
	for i := range fns {
		go func(i int) {
			n := measure(p3, "abufwriter", func() sp.Tlength {
				sz := newFile(t, fsls[i], fns[i], HASYNC, buf, FILESZ)
				err := ts.Remove(fns[i])
				assert.Nil(t, err, "Remove err %v", err)
				return sz
			})
			done <- n
		}(i)
	}
	n = 0
	for _ = range fns {
		n += <-done
	}
	ms = time.Since(start).Milliseconds()
	db.DPrintf(db.ALWAYS, "Total tpt bufwriter: %s took %vms (%s)", humanize.Bytes(uint64(n)), ms, test.TputStr(n, ms))
	ts.Shutdown()
}

func TestReadFilePerfSingle(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)
	fn := gopath.Join(pathname, "f")
	buf := test.NewBuf(WRITESZ)
	// Remove just in case it was left over from a previous run.
	ts.Remove(fn)
	sz := newFile(t, ts.FsLib, fn, HBUF, buf, SYNCFILESZ)
	p1, r := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.READER)
	assert.Nil(t, r)
	defer p1.Done()
	measure(p1, "reader", func() sp.Tlength {
		r, err := ts.OpenReader(fn)
		assert.Nil(t, err)
		n, err := test.Reader(t, r, buf, sz)
		assert.Nil(t, err)
		r.Close()
		return n
	})
	err := ts.Remove(fn)
	assert.Nil(t, err)
	p2, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.BUFREADER)
	assert.Nil(t, err)
	defer p2.Done()
	sz = newFile(t, ts.FsLib, fn, HBUF, buf, FILESZ)
	measure(p2, "bufreader", func() sp.Tlength {
		r, err := ts.OpenReader(fn)
		assert.Nil(t, err)
		br := bufio.NewReaderSize(r, sp.BUFSZ)
		n, err := test.Reader(t, br, buf, sz)
		assert.Nil(t, err)
		r.Close()
		return n
	})
	p3, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.ABUFREADER)
	assert.Nil(t, err)
	defer p3.Done()
	measure(p3, "readahead", func() sp.Tlength {
		r, err := ts.OpenAsyncReader(fn, 0)
		assert.Nil(t, err)
		n, err := test.Reader(t, r, buf, sz)
		assert.Nil(t, err)
		r.Close()
		return n
	})
	err = ts.Remove(fn)
	assert.Nil(t, err)
	ts.Shutdown()
}

func TestReadFilePerfMultiClient(t *testing.T) {
	ts := test.NewTstatePath(t, pathname)
	N_CLI := 10
	buf := test.NewBuf(WRITESZ)
	done := make(chan sp.Tlength)
	fns := make([]string, 0, N_CLI)
	fsls := make([]*fslib.FsLib, 0, N_CLI)
	for i := 0; i < N_CLI; i++ {
		fns = append(fns, gopath.Join(pathname, "f"+strconv.Itoa(i)))
		pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), i)
		fsl, err := fslib.NewFsLib(pcfg)
		assert.Nil(t, err)
		fsls = append(fsls, fsl)
	}
	// Remove just in case it was left over from a previous run.
	for _, fn := range fns {
		ts.Remove(fn)
		newFile(t, ts.FsLib, fn, HBUF, buf, SYNCFILESZ)
	}
	p1, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.READER)
	assert.Nil(t, err)
	defer p1.Done()
	start := time.Now()
	for i := range fns {
		go func(i int) {
			n := measure(p1, "reader", func() sp.Tlength {
				r, err := fsls[i].OpenReader(fns[i])
				assert.Nil(t, err)
				n, err := test.Reader(t, r, buf, SYNCFILESZ)
				assert.Nil(t, err)
				r.Close()
				return n
			})
			done <- n
		}(i)
	}
	n := sp.Tlength(0)
	for _ = range fns {
		n += <-done
	}
	ms := time.Since(start).Milliseconds()
	db.DPrintf(db.ALWAYS, "Total tpt reader: %s took %vms (%s)", humanize.Bytes(uint64(n)), ms, test.TputStr(n, ms))
	for _, fn := range fns {
		err := ts.Remove(fn)
		assert.Nil(ts.T, err)
		newFile(t, ts.FsLib, fn, HBUF, buf, FILESZ)
	}
	p2, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.BUFREADER)
	assert.Nil(t, err)
	defer p2.Done()
	start = time.Now()
	for i := range fns {
		go func(i int) {
			n := measure(p2, "bufreader", func() sp.Tlength {
				r, err := fsls[i].OpenReader(fns[i])
				assert.Nil(t, err)
				br := bufio.NewReaderSize(r, sp.BUFSZ)
				n, err := test.Reader(t, br, buf, FILESZ)
				assert.Nil(t, err)
				r.Close()
				return n
			})
			done <- n
		}(i)
	}
	n = 0
	for _ = range fns {
		n += <-done
	}
	ms = time.Since(start).Milliseconds()
	db.DPrintf(db.ALWAYS, "Total tpt bufreader: %s took %vms (%s)", humanize.Bytes(uint64(n)), ms, test.TputStr(n, ms))
	p3, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.ABUFREADER)
	assert.Nil(t, err)
	defer p3.Done()
	start = time.Now()
	for i := range fns {
		go func(i int) {
			n := measure(p3, "readabuf", func() sp.Tlength {
				r, err := fsls[i].OpenAsyncReader(fns[i], 0)
				assert.Nil(t, err)
				n, err := test.Reader(t, r, buf, FILESZ)
				assert.Nil(t, err)
				r.Close()
				return n
			})
			done <- n
		}(i)
	}
	n = 0
	for _ = range fns {
		n += <-done
	}
	ms = time.Since(start).Milliseconds()
	db.DPrintf(db.ALWAYS, "Total tpt abufreader: %s took %vms (%s)", humanize.Bytes(uint64(n)), ms, test.TputStr(n, ms))
	ts.Shutdown()
}

func newDir(t *testing.T, fsl *fslib.FsLib, dir string, n int) int {
	err := fsl.MkDir(dir, 0777)
	assert.Equal(t, nil, err)
	for i := 0; i < n; i++ {
		b := []byte("hello")
		_, err := fsl.PutFile(gopath.Join(dir, "f"+strconv.Itoa(i)), 0777, sp.OWRITE, b)
		assert.Nil(t, err)
	}
	return n
}

func TestDirCreatePerf(t *testing.T) {
	const N = 1000
	ts := test.NewTstatePath(t, pathname)
	dir := gopath.Join(pathname, "d")
	measuredir("create dir", 1, func() int {
		n := newDir(t, ts.FsLib, dir, N)
		return n
	})
	err := ts.RmDir(dir)
	assert.Nil(t, err)
	ts.Shutdown()
}

func lookuper(ts *test.Tstate, nclerk int, n int, dir string, nfile int, lip string, nds sp.Taddrs) {
	const NITER = 100 // 10000
	ch := make(chan bool)
	for c := 0; c < nclerk; c++ {
		go func(c int) {
			pcfg := proc.NewAddedProcEnv(ts.ProcEnv(), c)
			fsl, err := fslib.NewFsLib(pcfg)
			assert.Nil(ts.T, err)
			measuredir("lookup dir entry", NITER, func() int {
				for f := 0; f < nfile; f++ {
					_, err := fsl.Stat(gopath.Join(dir, "f"+strconv.Itoa(f)))
					assert.Nil(ts.T, err)
				}
				return nfile
			})
			ch <- true
		}(c)
	}
	for c := 0; c < nclerk; c++ {
		<-ch
	}
}

func TestDirReadPerf(t *testing.T) {
	const N = 10000
	const NFILE = 10
	const NCLERK = 1
	ts := test.NewTstatePath(t, pathname)
	dir := pathname + "d"
	n := newDir(t, ts.FsLib, dir, NFILE)
	assert.Equal(t, NFILE, n)
	measuredir("read dir", 1, func() int {
		n := 0
		ts.ProcessDir(dir, func(st *sp.Stat) (bool, error) {
			n += 1
			return false, nil
		})
		return n
	})
	lookuper(ts, 1, N, dir, NFILE, ts.GetLocalIP(), ts.NamedAddr())
	//lookuper(t, NCLERK, N, dir, NFILE)
	err := ts.RmDir(dir)
	assert.Nil(t, err)
	ts.Shutdown()
}

func TestRmDirPerf(t *testing.T) {
	const N = 5000
	ts := test.NewTstatePath(t, pathname)
	dir := gopath.Join(pathname, "d")
	n := newDir(t, ts.FsLib, dir, N)
	assert.Equal(t, N, n)
	measuredir("rm dir", 1, func() int {
		err := ts.RmDir(dir)
		assert.Nil(t, err)
		return N
	})
	ts.Shutdown()
}

func dump(t *testing.T) {
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, test.EtcdIP, "", "", false)
	fs, err := fsetcd.NewFsEtcd(pcfg.GetRealm(), pcfg.GetEtcdIP())
	assert.Nil(t, err)
	nd, err := fs.ReadDir(fsetcd.ROOT)
	assert.Nil(t, err)
	err = fs.Dump(0, nd, path.Path{}, fsetcd.ROOT)
	assert.Nil(t, err)
}

func TestLookupPerf(t *testing.T) {
	const N = 10
	const NFILE = 10
	ts := test.NewTstatePath(t, pathname)

	ts.RmDir(gopath.Join(pathname, "d0"))

	for d := 1; d < N; d++ {
		dir := pathname
		for i := 0; i < d; i++ {
			dir = gopath.Join(dir, "d"+strconv.Itoa(i))
			n := newDir(t, ts.FsLib, dir, NFILE)
			assert.Equal(t, NFILE, n)
		}
		//dump(t)
		label := fmt.Sprintf("stat dir %v nfile %v", dir, NFILE)
		measuredir(label, 100, func() int {
			_, err := ts.Stat(dir)
			assert.Nil(t, err)
			return 1
		})
		err := ts.RmDir(gopath.Join(pathname, "d0"))
		assert.Nil(t, err)
	}
	ts.Shutdown()
}
