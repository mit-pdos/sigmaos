package semclnt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/delay"
	"sigmaos/fslib"
	"sigmaos/semclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	WAIT_PATH = "name/wait"
)

func TestSemClntSimple(t *testing.T) {
	ts := test.MakeTstate(t)

	err := ts.MkDir(WAIT_PATH, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	fsl0, err := fslib.MakeFsLibAddr("sem0", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
	assert.Nil(ts.T, err, "fsl0")

	sem := semclnt.MakeSemClnt(ts.FsLib, WAIT_PATH+"/x")
	sem.Init(0)

	ch := make(chan bool)
	go func(ch chan bool) {
		sem := semclnt.MakeSemClnt(fsl0, WAIT_PATH+"/x")
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
	ts := test.MakeTstate(t)

	err := ts.MkDir(WAIT_PATH, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	fsl0, err := fslib.MakeFsLibAddr("sem0", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
	assert.Nil(ts.T, err, "fsl0")
	fsl1, err := fslib.MakeFsLibAddr("semd1", sp.ROOTREALM, ts.GetLocalIP(), ts.NamedAddr())
	assert.Nil(ts.T, err, "fsl1")

	for i := 0; i < 1000; i++ {
		sem := semclnt.MakeSemClnt(ts.FsLib, WAIT_PATH+"/x")
		sem.Init(0)

		ch := make(chan bool)

		go func(ch chan bool) {
			delay.Delay(10)
			sem := semclnt.MakeSemClnt(fsl0, WAIT_PATH+"/x")
			sem.Down()
			ch <- true
		}(ch)
		go func(ch chan bool) {
			delay.Delay(10)
			sem := semclnt.MakeSemClnt(fsl1, WAIT_PATH+"/x")
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
