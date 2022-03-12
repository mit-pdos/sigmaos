package semclnt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/delay"
	"ulambda/fslib"
	"ulambda/semclnt"
	"ulambda/test"
)

const (
	WAIT_PATH = "name/wait"
)

func TestSemClntSimple(t *testing.T) {
	ts := test.MakeTstate(t)

	err := ts.MkDir(WAIT_PATH, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	fsl0 := fslib.MakeFsLibAddr("sem0", fslib.Named())

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

	ts.Shutdown()
}

func TestSemClntConcur(t *testing.T) {
	ts := test.MakeTstate(t)

	err := ts.MkDir(WAIT_PATH, 0777)
	assert.Nil(ts.T, err, "Mkdir")
	fsl0 := fslib.MakeFsLibAddr("sem0", fslib.Named())
	fsl1 := fslib.MakeFsLibAddr("semd1", fslib.Named())

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
	ts.Shutdown()
}
