package fsux

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	fn      = sp.UX + "/~local/"
	FILESZ  = 50 * sp.MBYTE
	WRITESZ = 4096
)

func TestCompile(t *testing.T) {
}

func TestRoot(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	dirents, err := ts.GetDir(fn)
	assert.Nil(t, err, "GetDir")

	assert.NotEqual(t, 0, len(dirents))

	ts.Shutdown()
}

func TestFile(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	d := []byte("hello")
	_, err := ts.PutFile(fn+"f", 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(fn + "f")
	assert.Equal(t, string(d), string(d1))

	err = ts.Remove(fn + "f")
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func TestDir(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	err := ts.MkDir(fn+"d1", 0777)
	assert.Equal(t, nil, err)
	d := []byte("hello")

	dirents, err := ts.GetDir(fn + "d1")
	assert.Nil(t, err, "GetDir")

	assert.Equal(t, 0, len(dirents))

	_, err = ts.PutFile(fn+"d1/f", 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(fn + "d1/f")
	assert.Equal(t, string(d), string(d1))

	err = ts.Remove(fn + "d1/f")
	assert.Equal(t, nil, err)

	err = ts.Remove(fn + "d1")
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func newfile(t *testing.T, name string) {
	CNT := 500
	buf := test.NewBuf(sp.BUFSZ)
	start := time.Now()
	fd, err := syscall.Open(name, syscall.O_CREAT|syscall.O_EXCL|syscall.O_WRONLY, 0666)
	assert.Nil(t, err)
	for i := 0; i < CNT; i++ {
		n, err := syscall.Pwrite(fd, buf, int64(i*sp.BUFSZ))
		assert.Nil(t, err)
		assert.Equal(t, sp.BUFSZ, n)
	}
	syscall.Fsync(fd)
	syscall.Close(fd)
	ms := time.Since(start).Milliseconds()
	sz := uint64(CNT * len(buf))
	fmt.Printf("%s took %vms (%s)\n", humanize.Bytes(sz), ms, test.TputStr(sp.Tlength(sz), ms))
	os.Remove(name)
}

func TestFsPerfSingle(t *testing.T) {
	newfile(t, "xxx")
}

func TestFsPerfMulti(t *testing.T) {

	var done sync.WaitGroup
	done.Add(2)
	go func() {
		newfile(t, "xxx")
		done.Done()
	}()
	go func() {
		newfile(t, "yyy")
		done.Done()
	}()
	done.Wait()
}

func writer(t *testing.T, ch chan error, pe *proc.ProcEnv, idx int) {
	fsl, err := sigmaclnt.NewFsLib(pe)
	assert.Nil(t, err)
	fn := sp.UX + "~local/file-" + string(pe.GetPrincipal().GetID()) + "-" + strconv.Itoa(idx)
	stop := false
	nfile := 0
	for !stop {
		select {
		case <-ch:
			stop = true
		default:
			db.DPrintf(db.ALWAYS, "Writer %v remove", idx)
			if err := fsl.Remove(fn); serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.ALWAYS, "Writer %v remove async done err %v", idx, err)
				break
			}
			db.DPrintf(db.ALWAYS, "Writer %v create async", idx)
			w, err := fsl.CreateAsyncWriter(fn, 0777, sp.OWRITE)
			if err != nil {
				db.DPrintf(db.ALWAYS, "Writer %v create async done err %v", idx, err)
				assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable), "Err code %v", err)
				break
			}
			db.DPrintf(db.ALWAYS, "Writer %v writer", idx)
			nfile += 1
			buf := test.NewBuf(WRITESZ)
			if err := test.Writer(t, w, buf, FILESZ); err != nil {
				db.DPrintf(db.ALWAYS, "Writer %v writer done err %v", idx, err)
				break
			}
			db.DPrintf(db.ALWAYS, "Writer %v close", idx)
			if err := w.Close(); err != nil {
				db.DPrintf(db.ALWAYS, "Writer %v close done err %v", idx, err)
				assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))
				break
			}
			db.DPrintf(db.ALWAYS, "Writer %v close done", idx)
		}
	}
	assert.True(t, nfile >= 3) // a bit arbitrary
	fsl.Remove(fn)
	fsl.Close()
}

func TestWriteCrash1x1(t *testing.T) {
	const (
		N        = 1
		NCRASH   = 1
		CRASHSRV = 1000000
	)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ch := make(chan error)

	for i := 0; i < N; i++ {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		err := ts.MintAndSetToken(pe)
		assert.Nil(t, err)
		go writer(ts.T, ch, pe, i)
	}

	crashchan := make(chan bool)
	l := &sync.Mutex{}
	for i := 0; i < NCRASH; i++ {
		go ts.CrashServer(sp.UXREL, (i+1)*CRASHSRV, l, crashchan)
	}

	for i := 0; i < NCRASH; i++ {
		<-crashchan
	}

	db.DPrintf(db.TEST, "Done waiting for crashes")

	for i := 0; i < N; i++ {
		ch <- nil
		db.DPrintf(db.TEST, "Done stopping writer #%v", i)
	}

	ts.Shutdown()
}

func TestWriteCrash1x20(t *testing.T) {
	const (
		N        = 20
		NCRASH   = 1
		CRASHSRV = 1000000
	)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ch := make(chan error)

	for i := 0; i < N; i++ {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		err := ts.MintAndSetToken(pe)
		assert.Nil(t, err)
		go writer(ts.T, ch, pe, i)
	}

	crashchan := make(chan bool)
	l := &sync.Mutex{}
	for i := 0; i < NCRASH; i++ {
		go ts.CrashServer(sp.UXREL, (i+1)*CRASHSRV, l, crashchan)
	}

	for i := 0; i < NCRASH; i++ {
		<-crashchan
	}

	db.DPrintf(db.TEST, "Done waiting for crashes")

	for i := 0; i < N; i++ {
		ch <- nil
		db.DPrintf(db.TEST, "Done stopping writer #%v", i)
	}

	ts.Shutdown()
}

func TestWriteCrash5x1(t *testing.T) {
	const (
		N        = 20
		NCRASH   = 5
		CRASHSRV = 1000000
	)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ch := make(chan error)

	for i := 0; i < N; i++ {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		err := ts.MintAndSetToken(pe)
		assert.Nil(t, err)
		go writer(ts.T, ch, pe, i)
	}

	crashchan := make(chan bool)
	l := &sync.Mutex{}
	for i := 0; i < NCRASH; i++ {
		go ts.CrashServer(sp.UXREL, (i+1)*CRASHSRV, l, crashchan)
	}

	for i := 0; i < NCRASH; i++ {
		<-crashchan
	}

	for i := 0; i < N; i++ {
		ch <- nil
	}

	ts.Shutdown()
}

func TestWriteCrash5x20(t *testing.T) {
	const (
		N        = 20
		NCRASH   = 5
		CRASHSRV = 1000000
	)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ch := make(chan error)

	for i := 0; i < N; i++ {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		err := ts.MintAndSetToken(pe)
		assert.Nil(t, err)
		go writer(ts.T, ch, pe, i)
	}

	crashchan := make(chan bool)
	l := &sync.Mutex{}
	for i := 0; i < NCRASH; i++ {
		go ts.CrashServer(sp.UXREL, (i+1)*CRASHSRV, l, crashchan)
	}

	for i := 0; i < NCRASH; i++ {
		<-crashchan
	}

	for i := 0; i < N; i++ {
		ch <- nil
	}

	ts.Shutdown()
}
