package semaphore_test

import (
	"flag"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/coordination/semaphore"
	"sigmaos/util/rand"
)

const (
	SEMDIR = "bardir"
)

var pathname string // e.g., --path "name/msched/sp.LOCAL/"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func Delay(maxms int64) {
	ms := rand.Int64(maxms)
	db.DPrintf(db.DELAY, "Delay to %vms\n", ms)
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func TestCompile(t *testing.T) {
}

func TestSemaphoreSimple(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pn := filepath.Join(pathname, SEMDIR)

	db.DPrintf(db.TEST, "pn %v\n", pn)

	err := ts.MkDir(pn, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl0, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	assert.Nil(ts.T, err, "fsl0")

	sem := semaphore.NewSemaphore(ts.FsLib, pn+"/x")
	sem.Init(0)

	ch := make(chan bool)
	go func(ch chan bool) {
		sem := semaphore.NewSemaphore(fsl0, pn+"/x")
		sem.Down()
		ch <- true
	}(ch)

	time.Sleep(100 * time.Millisecond)

	select {
	case ok := <-ch:
		assert.False(ts.T, ok, "down should be blocked")
	default:
	}

	sem.Up()

	ok := <-ch
	assert.True(ts.T, ok, "down")

	err = ts.RmDir(pn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestSemaphoreConcur(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	pn := filepath.Join(pathname, SEMDIR)
	err := ts.MkDir(pn, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	pe1 := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl0, err := sigmaclnt.NewFsLib(pe1, dialproxyclnt.NewDialProxyClnt(pe1))
	assert.Nil(ts.T, err, "fsl0")
	pe2 := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl1, err := sigmaclnt.NewFsLib(pe2, dialproxyclnt.NewDialProxyClnt(pe2))
	assert.Nil(ts.T, err, "fsl1")

	for i := 0; i < 100; i++ {
		sem := semaphore.NewSemaphore(ts.FsLib, pn+"/x")
		sem.Init(0)

		ch := make(chan bool)

		go func(ch chan bool) {
			Delay(200)
			sem := semaphore.NewSemaphore(fsl0, pn+"/x")
			sem.Down()
			ch <- true
		}(ch)
		go func(ch chan bool) {
			Delay(200)
			sem := semaphore.NewSemaphore(fsl1, pn+"/x")
			sem.Up()
			ch <- true
		}(ch)

		for i := 0; i < 2; i++ {
			<-ch
		}
	}
	err = ts.RmDir(pn)
	assert.Nil(t, err, "RmDir: %v", err)
	ts.Shutdown()
}

func TestSemaphoreFail(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pn := filepath.Join(pathname, SEMDIR)
	err := ts.MkDir(pn, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	pe := proc.NewAddedProcEnv(ts.ProcEnv())

	sc1, err := sigmaclnt.NewSigmaClnt(pe)

	li, err := sc1.LeaseClnt.AskLease(pn+"/bar", fsetcd.LeaseTTL)
	assert.Nil(t, err, "Error AskLease: %v", err)

	sem := semaphore.NewSemaphore(sc1.FsLib, pn+"/bar")
	sem.InitLease(0777, li.Lease())

	ch := make(chan bool)
	go func(ch chan bool) {
		sem := semaphore.NewSemaphore(ts.FsLib, pn+"/bar")
		sem.Down()
		ch <- true
	}(ch)

	time.Sleep(100 * time.Millisecond)

	select {
	case ok := <-ch:
		assert.False(ts.T, ok, "down should be blocked")
	default:
	}

	err = sc1.Close()

	assert.Nil(ts.T, err)

	ok := <-ch
	assert.True(ts.T, ok, "down")

	err = ts.RmDir(pn)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}
