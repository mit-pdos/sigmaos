package sigmaclntclnt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
}

func TestStat(t *testing.T) {
	ts := test.NewTstateAll(t)

	st, err := ts.Stat("name/")
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Stat %v err %v\n", st, err)

	ts.Shutdown()
}
