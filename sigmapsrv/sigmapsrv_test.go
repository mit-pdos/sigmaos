package sigmapsrv_test

import (
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/netproxyclnt"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/~local/"
var withmarshal bool

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
	flag.BoolVar(&withmarshal, "withmarshal", false, "With marshal?")
}

const (
	KBYTE      = 1 << 10
	NRUNS      = 3
	SYNCFILESZ = 100 * KBYTE
	//	SYNCFILESZ = 250 * KBYTE
	// SYNCFILESZ = WRITESZ
	FILESZ  = 100 * sp.MBYTE
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

func measuredir(msg string, nruns int, dir string, f func() int) {
	tot := float64(0)
	n := 0
	for i := 0; i < nruns; i++ {
		start := time.Now()
		n += f()
		ms := time.Since(start).Milliseconds()
		tot += float64(ms)
	}
	s := tot / 1000
	db.DPrintf(db.TEST, "%v: %v nops %d took %vms (%.1f op/s)", msg, dir, n, tot, float64(n)/s)
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
		w, err := fsl.CreateBufWriter(fn, 0777)
		assert.Nil(t, err, "Error Create writer: %v", err)
		err = test.Writer(t, w, buf, sz)
		assert.Nil(t, err, "Err writer %v", err)
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

func TestCompile(t *testing.T) {
}

func TestWriteFilePerfSingle(t *testing.T) {
	if !assert.NotEqual(t, pathname, sp.NAMED, "Writing to named will trigger errors, because the buf size is too large for etcd's maximum write size") {
		return
	}
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	fn := filepath.Join(pathname, "f")
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
	if !assert.NotEqual(t, pathname, sp.NAMED, "Writing to named will trigger errors, because the buf size is too large for etcd's maximum write size") {
		return
	}
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	N_CLI := 10
	buf := test.NewBuf(WRITESZ)
	done := make(chan sp.Tlength)
	fns := make([]string, 0, N_CLI)
	fsls := make([]*fslib.FsLib, 0, N_CLI)
	for i := 0; i < N_CLI; i++ {
		fns = append(fns, filepath.Join(pathname, "f"+strconv.Itoa(i)))
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
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
	if !assert.NotEqual(t, pathname, sp.NAMED, "Writing to named will trigger errors, because the buf size is too large for etcd's maximum write size") {
		return
	}

	var sz sp.Tlength
	var err error

	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	fn := filepath.Join(pathname, "f")
	buf := test.NewBuf(WRITESZ)

	// Remove just in case it was left over from a previous run.
	ts.Remove(fn)
	sz = newFile(t, ts.FsLib, fn, HBUF, buf, SYNCFILESZ)

	p1, r := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.READER)
	assert.Nil(t, r)
	measure(p1, "reader", func() sp.Tlength {
		pn := fn
		if test.Withs3pathclnt {
			pn0, ok := sp.S3ClientPath(fn)
			assert.True(t, ok)
			pn = pn0
		}
		r, err := ts.OpenReader(pn)
		assert.Nil(t, err)
		n, err := test.Reader(t, r, buf, sz)
		assert.Nil(t, err)
		r.Close()
		return n
	})
	p1.Done()

	err = ts.Remove(fn)
	assert.Nil(t, err)
	sz = newFile(t, ts.FsLib, fn, HBUF, buf, FILESZ)

	p2, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.BUFREADER)
	assert.Nil(t, err)
	measure(p2, "bufreader", func() sp.Tlength {
		pn := fn
		if test.Withs3pathclnt {
			pn0, ok := sp.S3ClientPath(fn)
			assert.True(t, ok)
			pn = pn0
		}
		r, err := ts.OpenBufReader(pn)
		n, err := test.Reader(t, r, buf, sz)
		assert.Nil(t, err)
		r.Close()
		return n
	})
	p2.Done()

	err = ts.Remove(fn)
	assert.Nil(t, err)
	sz = newFile(t, ts.FsLib, fn, HBUF, buf, FILESZ)

	p3, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, perf.ABUFREADER)
	assert.Nil(t, err)
	measure(p3, "readahead", func() sp.Tlength {
		pn := fn
		if test.Withs3pathclnt {
			pn0, ok := sp.S3ClientPath(fn)
			assert.True(t, ok)
			pn = pn0
		}
		r, err := ts.OpenAsyncReader(pn, 0)
		assert.Nil(t, err)
		n, err := test.Reader(t, r, buf, sz)
		assert.Nil(t, err)
		r.Close()
		return n
	})
	p3.Done()

	err = ts.Remove(fn)
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestReadFilePerfMultiClient(t *testing.T) {
	if !assert.NotEqual(t, pathname, sp.NAMED, "Writing to named will trigger errors, because the buf size is too large for etcd's maximum write size") {
		return
	}
	const (
		NTRIAL = 3
	)

	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	N_CLI := 4
	buf := test.NewBuf(WRITESZ)
	done := make(chan sp.Tlength)
	fns := make([]string, 0, N_CLI)
	fsls := make([]*fslib.FsLib, 0, N_CLI)
	for i := 0; i < N_CLI; i++ {
		fns = append(fns, filepath.Join(pathname, "f"+strconv.Itoa(i)))
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
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
				n := sp.Tlength(0)
				for j := 0; j < NTRIAL; j++ {
					r, err := fsls[i].OpenReader(fns[i])
					assert.Nil(t, err)
					n2, err := test.Reader(t, r, buf, SYNCFILESZ)
					assert.Nil(t, err)
					n += n2
					r.Close()
				}
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
	defer p2.Done()
	assert.Nil(t, err)
	start = time.Now()
	for i := range fns {
		go func(i int) {
			n := measure(p2, "bufreader", func() sp.Tlength {
				n := sp.Tlength(0)
				for j := 0; j < NTRIAL; j++ {
					r, err := fsls[i].OpenBufReader(fns[i])
					assert.Nil(t, err)
					n2, err := test.Reader(t, r, buf, FILESZ)
					assert.Nil(t, err)
					n += n2
					r.Close()
				}
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
				n := sp.Tlength(0)
				for j := 0; j < NTRIAL; j++ {
					r, err := fsls[i].OpenAsyncReader(fns[i], 0)
					assert.Nil(t, err)
					n2, err := test.Reader(t, r, buf, FILESZ)
					assert.Nil(t, err)
					n += n2
					r.Close()
				}
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
		_, err := fsl.PutFile(filepath.Join(dir, "f"+strconv.Itoa(i)), 0777, sp.OWRITE, b)
		assert.Nil(t, err)
	}
	return n
}

func newDirLeased(ts *test.Tstate, dir string, n int) int {
	err := ts.MkDir(dir, 0777)
	assert.Nil(ts.T, err)

	li, err := ts.LeaseClnt.AskLease(dir, fsetcd.LeaseTTL)
	assert.Nil(ts.T, err, "Error AskLease: %v", err)

	for i := 0; i < n; i++ {
		b := []byte("hello")
		_, err := ts.PutLeasedFile(filepath.Join(dir, "f"+strconv.Itoa(i)), 0777, sp.OWRITE, li.Lease(), b)
		assert.Nil(ts.T, err)
	}
	return n
}

func TestDirCreatePerf(t *testing.T) {
	const N = 1000
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dir := filepath.Join(pathname, "d")
	measuredir("create dir", 1, dir, func() int {
		n := newDir(t, ts.FsLib, dir, N)
		return n
	})
	err := ts.RmDir(dir)
	assert.Nil(t, err)
	ts.Shutdown()
}

func fileRename(t *testing.T, fsl *fslib.FsLib, dir string, n int) int {
	newDir(t, fsl, dir, 100)
	b := []byte("hello")
	_, err := fsl.PutFile(filepath.Join(dir, "f"), 0777, sp.OWRITE, b)
	assert.Nil(t, err)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			err := fsl.Rename(filepath.Join(dir, "f"), filepath.Join(dir, "g"))
			assert.Nil(t, err)
		} else {
			err := fsl.Rename(filepath.Join(dir, "g"), filepath.Join(dir, "f"))
			assert.Nil(t, err)
		}
	}
	return n
}

func TestFileRenamePerf(t *testing.T) {
	const N = 1000
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dir := filepath.Join(pathname, "d")
	measuredir("rename", 1, dir, func() int {
		n := fileRename(t, ts.FsLib, dir, N)
		return n
	})
	err := ts.RmDir(dir)
	assert.Nil(t, err)
	ts.Shutdown()
}

func lookuper(ts *test.Tstate, nclerk int, n int, dir string, nfile int, lip sp.Tip) {
	const NITER = 100 // 10000
	ch := make(chan bool)
	for c := 0; c < nclerk; c++ {
		go func(c int) {
			pe := proc.NewAddedProcEnv(ts.ProcEnv())
			fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
			assert.Nil(ts.T, err)
			measuredir("lookup dir entry", NITER, dir, func() int {
				for f := 0; f < nfile; f++ {
					_, err := fsl.Stat(filepath.Join(dir, "f"+strconv.Itoa(f)))
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
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dir := pathname + "d"
	n := newDir(t, ts.FsLib, dir, NFILE)
	assert.Equal(t, NFILE, n)
	measuredir("read dir", 1, dir, func() int {
		n := 0
		ts.ProcessDir(dir, func(st *sp.Stat) (bool, error) {
			n += 1
			return false, nil
		})
		return n
	})
	lookuper(ts, 1, N, dir, NFILE, ts.ProcEnv().GetInnerContainerIP())
	//lookuper(t, NCLERK, N, dir, NFILE)
	err := ts.RmDir(dir)
	assert.Nil(t, err)
	ts.Shutdown()
}

func getDirPerf(t *testing.T, leased bool) {
	const (
		NFILE = 100
		N     = 1000

		DIRNAME = "d"
	)

	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	var st0 *fsetcd.PstatsSnapshot
	if pathname == sp.NAMED {
		st, err := ts.ReadPstats()
		assert.Nil(t, err)
		st0 = st
	}

	dir := filepath.Join(pathname, "d")
	if leased {
		n := newDirLeased(ts, dir, NFILE)
		assert.Equal(t, NFILE, n)
	} else {
		n := newDir(ts.T, ts.FsLib, dir, NFILE)
		assert.Equal(t, NFILE, n)
	}

	if st0 != nil {
		st, err := ts.ReadPstats()
		db.DPrintf(db.TEST, "pstats: %v", st.Counters[DIRNAME]-st0.Counters[DIRNAME])
		assert.Nil(t, err)
		st0 = st
	}

	measuredir(fmt.Sprintf("GetDir %t", leased), N, dir, func() int {
		sts, err := ts.GetDir(dir)
		assert.Nil(t, err)
		assert.Equal(t, NFILE, len(sts))
		return N
	})

	if st0 != nil {
		st, err := ts.ReadPstats()
		db.DPrintf(db.TEST, "pstats: %v", st.Counters[DIRNAME]-st0.Counters[DIRNAME])
		assert.Nil(t, err)
	}

	err = ts.RmDir(dir)
	assert.Nil(t, err)
	ts.Shutdown()
}

func TestGetDirPerfLeasedFile(t *testing.T) {
	getDirPerf(t, true)
}

func TestGetDirPerfFile(t *testing.T) {
	getDirPerf(t, false)
}

func TestRmDirPerf(t *testing.T) {
	const N = 5000
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	dir := filepath.Join(pathname, "d")
	n := newDir(t, ts.FsLib, dir, N)
	assert.Equal(t, N, n)
	measuredir("rm dir", 1, dir, func() int {
		err := ts.RmDir(dir)
		assert.Nil(t, err)
		return N
	})
	ts.Shutdown()
}

func TestLookupDepthPerf(t *testing.T) {
	const N = 10
	const NFILE = 10
	const NOP = 10000
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts.RmDir(filepath.Join(pathname, "d0"))

	for d := 1; d < N; d++ {
		dir := pathname
		for i := 0; i < d; i++ {
			dir = filepath.Join(dir, "d"+strconv.Itoa(i))
			n := newDir(t, ts.FsLib, dir, NFILE)
			assert.Equal(t, NFILE, n)
		}
		label := fmt.Sprintf("stat dir %v nfile %v", dir, NFILE)
		measuredir(label, NOP, dir, func() int {
			_, err := ts.Stat(dir)
			assert.Nil(t, err)
			return 1
		})
		err := ts.RmDir(filepath.Join(pathname, "d0"))
		assert.Nil(t, err)
	}
	ts.Shutdown()
}

func TestLookupConcurPerf(t *testing.T) {
	const N = 1
	const NFILE = 10
	const NGO = 10
	const NTRIAL = 100
	ts, err1 := test.NewTstatePath(t, pathname)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts.RmDir(filepath.Join(pathname, "d0"))

	dir := pathname
	for d := 0; d < N; d++ {
		dir = filepath.Join(dir, "d"+strconv.Itoa(d))
		n := newDir(t, ts.FsLib, dir, NFILE)
		assert.Equal(t, NFILE, n)
	}
	ndMnt, err := ts.GetNamedEndpoint()
	assert.Nil(t, err, "GetNamedEndpoint: %v", err)
	// dump(t)
	done := make(chan int)
	fsls := make([][]*fslib.FsLib, 0, NGO)
	for i := 0; i < NGO; i++ {
		fsl2 := make([]*fslib.FsLib, 0, NTRIAL)
		for j := 0; j < NTRIAL; j++ {
			pe := proc.NewAddedProcEnv(ts.ProcEnv())
			pe.NamedEndpointProto = ndMnt.TendpointProto
			fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
			assert.Nil(t, err)
			fsl2 = append(fsl2, fsl)
		}
		fsls = append(fsls, fsl2)
	}

	for i := 0; i < NGO; i++ {
		go func(i int) {
			label := fmt.Sprintf("stat dir %v nfile %v ntrial %v", dir, NFILE, NTRIAL)
			measuredir(label, 1, dir, func() int {
				for j := 0; j < NTRIAL; j++ {
					_, err := fsls[i][j].Stat(dir)
					assert.Nil(t, err, "stat err %v", err)
				}
				return NTRIAL
			})
			done <- i
		}(i)
	}

	for _ = range fsls {
		<-done
	}

	err = ts.RmDir(filepath.Join(pathname, "d0"))
	assert.Nil(t, err)

	ts.Shutdown()
}

func TestLookupMultiMount(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	// Running a proc forces sigmaos to create uprocds and rpc special file
	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", 0), "name/"})
	err = ts.Spawn(a)
	assert.Nil(ts.T, err, "Spawn")
	_, err = ts.WaitExit(a.GetPid())
	assert.Nil(ts.T, err, "WaitExit error")

	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	//mnt, err := ts.GetNamedEndpoint()
	//assert.Nil(ts.T, err)
	//pe.NamedEndpointProto = mnt.GetProto()
	sts, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	kernelId := sts[0].Name

	sts, err = ts.GetDir(filepath.Join(sp.SCHEDD, kernelId, sp.UPROCDREL))
	assert.Nil(t, err)
	uprocdpid := sts[0].Name

	db.DPrintf(db.TEST, "kernelid %v %v\n", kernelId, uprocdpid)

	pe.NamedEndpointProto = nil
	fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
	assert.Nil(t, err)

	// cache named, which is typically the case
	_, err = fsl.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	s := time.Now()
	pn := filepath.Join(sp.SCHEDD, kernelId, rpc.RPC)
	// pn := filepath.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, uprocdpid, rpc.RPC)
	db.DPrintf(db.TEST, "Stat %v start %v\n", fsl.ClntId(), pn)
	_, err = fsl.Stat(pn)
	db.DPrintf(db.TEST, "Stat %v done %v took %v\n", fsl.ClntId(), pn, time.Since(s))
	assert.Nil(t, err)
	ts.Shutdown()
}

func TestColdPathMicro(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	sts, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	pe.KernelID = sts[0].Name

	pn := filepath.Join(sp.UX, "~local", "mr-intermediate")

	var max time.Duration
	var tot time.Duration
	const N = 1
	for i := 0; i < N; i++ {
		fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
		assert.Nil(t, err)
		db.DPrintf(db.TEST, "MkDir %v start %v", fsl.ClntId(), pn)
		s := time.Now()
		err = fsl.MkDir(pn, 0777)
		assert.Nil(t, err)
		d := time.Since(s)
		if d > max {
			max = d
		}
		tot += d
		fsl.RmDir(pn)
		fsl.Close()
	}
	db.DPrintf(db.TEST, "MkDir done %v took avg %v max %v", pn, tot/N, max)
	ts.Shutdown()
}

func TestColdAttach(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	sts, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	pe.KernelID = sts[0].Name

	pn := filepath.Join(sp.SCHEDD, pe.KernelID)
	ep, err := ts.ReadEndpoint(pn)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "endpoint %v", ep)

	var max time.Duration
	var tot time.Duration
	const N = 1
	for i := 0; i < N; i++ {
		fsl, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
		assert.Nil(t, err)
		pn = filepath.Join(pn, rpc.RPC)
		start := time.Now()
		err = fsl.MountTree(ep, rpc.RPC, pn)
		assert.Nil(t, err)
		d := time.Since(start)
		db.DPrintf(db.TEST, "Mount schedd [%v] %v as %v time %v", ep, rpc.RPC, pn, d)
		if d > max {
			max = d
		}
		tot += d
		fsl.Close()
	}
	db.DPrintf(db.TEST, "MountTree avg %v max %v", tot/N, max)
	ts.Shutdown()
}
