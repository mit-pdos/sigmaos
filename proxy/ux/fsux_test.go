package fsux

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
)

const (
	fn      = sp.UX + "/" + sp.LOCAL + "/"
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

func writer(t *testing.T, ch chan struct{}, pe *proc.ProcEnv, idx int) {
	fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	assert.Nil(t, err)
	fn := sp.UX + sp.LOCAL + "/file-" + string(pe.GetPrincipal().GetID()) + "-" + strconv.Itoa(idx)
	stop := false
	ncrash := 0
	for !stop {
		select {
		case <-ch:
			stop = true
		default:
			if err := fsl.Remove(fn); serr.IsErrCode(err, serr.TErrUnreachable) {
				ncrash += 1
				break
			}
			w, err := fsl.CreateBufWriter(fn, 0777)
			if err != nil {
				assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable), "Err code %v", err)
				ncrash += 1
				break
			}
			db.DPrintf(db.TEST, "created %v %d\n", fn, ncrash)
			buf := test.NewBuf(WRITESZ)
			if err := test.Writer(t, w, buf, FILESZ); err != nil {
				ncrash += 1
				break
			}
			if err := w.Close(); err != nil {
				assert.True(t, serr.IsErrCode(err, serr.TErrUnreachable))
				ncrash += 1
				break
			}
		}
	}
	assert.True(t, ncrash >= 1)
	fsl.Remove(fn)
	fsl.Close()
}

func TestWriteCrash5x20(t *testing.T) {
	const (
		N        = 5
		NCRASH   = 5
		CRASHSRV = 1000000
		T        = 2000
	)

	fn := sp.NAMED + fmt.Sprintf("crashux%d.sem", 0)
	e0 := crash.NewEventPath(crash.UX_CRASH, 0, 1.0, fn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e0))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ch := make(chan struct{})
	for i := 0; i < N; i++ {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		go writer(ts.T, ch, pe, i)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < NCRASH; i++ {
			fn = sp.NAMED + fmt.Sprintf("crashux%d.sem", i+1)
			e1 := crash.NewEventPath(crash.UX_CRASH, 0, 1.0, fn)
			ts.CrashUx(e0, e1)
			e0 = e1
			time.Sleep(T * time.Millisecond)
		}
	}()
	wg.Wait()

	for i := 0; i < N; i++ {
		ch <- struct{}{}
	}

	ts.Shutdown()
}
