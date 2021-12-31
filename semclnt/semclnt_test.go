package semclnt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/semclnt"
)

const (
	WAIT_PATH = "name/wait"
)

type Tstate struct {
	t *testing.T
	*kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemNamed("semclnt_test", "..")
	return ts
}

func TestSemClnt(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir(WAIT_PATH, 0777)
	assert.Nil(ts.t, err, "Mkdir")
	fsl0 := fslib.MakeFsLibAddr("sem0", fslib.Named())
	fsl1 := fslib.MakeFsLibAddr("semd1", fslib.Named())

	for i := 0; i < 100; i++ {
		sem := semclnt.MakeSemClnt(ts.FsLib, WAIT_PATH+"/x")
		sem.Init()

		ch := make(chan bool)

		go func(ch chan bool) {
			sem := semclnt.MakeSemClnt(fsl0, WAIT_PATH+"/x")
			sem.Down()
			ch <- true
		}(ch)
		go func(ch chan bool) {
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
