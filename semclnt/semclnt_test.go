package semclnt_test

import (
	"flag"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/delay"
	"sigmaos/fsetcd"
	"sigmaos/netproxyclnt"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SEMDIR = "semdir"
)

var pathname string // e.g., --path "name/schedd/~local/"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func TestCompile(t *testing.T) {
}

func TestSemClntSimple(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pn := path.Join(pathname, SEMDIR)

	db.DPrintf(db.TEST, "pn %v\n", pn)

	err := ts.MkDir(pn, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl0, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
	assert.Nil(ts.T, err, "fsl0")

	sem := semclnt.NewSemClnt(ts.FsLib, pn+"/x")
	sem.Init(0)

	ch := make(chan bool)
	go func(ch chan bool) {
		sem := semclnt.NewSemClnt(fsl0, pn+"/x")
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

func TestSemClntConcur(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	pn := path.Join(pathname, SEMDIR)
	err := ts.MkDir(pn, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	pe1 := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl0, err := sigmaclnt.NewFsLib(pe1, netproxyclnt.NewNetProxyClnt(pe1))
	assert.Nil(ts.T, err, "fsl0")
	pe2 := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl1, err := sigmaclnt.NewFsLib(pe2, netproxyclnt.NewNetProxyClnt(pe2))
	assert.Nil(ts.T, err, "fsl1")

	for i := 0; i < 100; i++ {
		sem := semclnt.NewSemClnt(ts.FsLib, pn+"/x")
		sem.Init(0)

		ch := make(chan bool)

		go func(ch chan bool) {
			delay.Delay(200)
			sem := semclnt.NewSemClnt(fsl0, pn+"/x")
			sem.Down()
			ch <- true
		}(ch)
		go func(ch chan bool) {
			delay.Delay(200)
			sem := semclnt.NewSemClnt(fsl1, pn+"/x")
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

func TestSemClntFail(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	pn := path.Join(pathname, SEMDIR)
	err := ts.MkDir(pn, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	pe := proc.NewAddedProcEnv(ts.ProcEnv())

	sc1, err := sigmaclnt.NewSigmaClnt(pe)

	li, err := sc1.LeaseClnt.AskLease(pn+"/x", fsetcd.LeaseTTL)
	assert.Nil(t, err, "Error AskLease: %v", err)

	sem := semclnt.NewSemClnt(sc1.FsLib, pn+"/x")
	sem.InitLease(0777, li.Lease())

	ch := make(chan bool)
	go func(ch chan bool) {
		sem := semclnt.NewSemClnt(ts.FsLib, pn+"/x")
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
