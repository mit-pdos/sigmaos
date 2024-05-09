package semclnt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/delay"
	"sigmaos/netproxyclnt"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	"sigmaos/test"
)

const (
	WAIT_PATH = "name/wait"
)

func TestCompile(t *testing.T) {
}

func TestSemClntSimple(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	err := ts.MkDir(WAIT_PATH, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl0, err := sigmaclnt.NewFsLib(pe, netproxyclnt.NewNetProxyClnt(pe, nil))
	assert.Nil(ts.T, err, "fsl0")

	sem := semclnt.NewSemClnt(ts.FsLib, WAIT_PATH+"/x")
	sem.Init(0)

	ch := make(chan bool)
	go func(ch chan bool) {
		sem := semclnt.NewSemClnt(fsl0, WAIT_PATH+"/x")
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

	err = ts.RmDir(WAIT_PATH)
	assert.Nil(t, err, "RmDir: %v", err)

	ts.Shutdown()
}

func TestSemClntConcur(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	err := ts.MkDir(WAIT_PATH, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	pe1 := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl0, err := sigmaclnt.NewFsLib(pe1, netproxyclnt.NewNetProxyClnt(pe1, nil))
	assert.Nil(ts.T, err, "fsl0")
	pe2 := proc.NewAddedProcEnv(ts.ProcEnv())
	fsl1, err := sigmaclnt.NewFsLib(pe2, netproxyclnt.NewNetProxyClnt(pe2, nil))
	assert.Nil(ts.T, err, "fsl1")

	for i := 0; i < 100; i++ {
		sem := semclnt.NewSemClnt(ts.FsLib, WAIT_PATH+"/x")
		sem.Init(0)

		ch := make(chan bool)

		go func(ch chan bool) {
			delay.Delay(200)
			sem := semclnt.NewSemClnt(fsl0, WAIT_PATH+"/x")
			sem.Down()
			ch <- true
		}(ch)
		go func(ch chan bool) {
			delay.Delay(200)
			sem := semclnt.NewSemClnt(fsl1, WAIT_PATH+"/x")
			sem.Up()
			ch <- true
		}(ch)

		for i := 0; i < 2; i++ {
			<-ch
		}
	}
	err = ts.RmDir(WAIT_PATH)
	assert.Nil(t, err, "RmDir: %v", err)
	ts.Shutdown()
}
