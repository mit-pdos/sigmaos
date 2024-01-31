package sigmaclntclnt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
}

func TestClose(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	go func() {
		err := ts.ReadDirWait("name/", func([]*sp.Stat) bool { return true })
		assert.NotNil(t, err)
	}()

	go func() {
		err := ts.ReadDirWait("name/", func([]*sp.Stat) bool { return true })
		assert.NotNil(t, err)
	}()

	st, err := ts.Stat("name/")

	ts.Close()

	db.DPrintf(db.TEST, "Stat %v err %v\n", st, err)

	ts.Shutdown()
}
