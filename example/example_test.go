package example_test

import (
	// Go imports:
	"log"
	"testing"

	// External imports:
	"github.com/stretchr/testify/assert"

	// SigmaOS imports:
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestExerciseNamed(t *testing.T) {
	dir := sp.NAMED
	ts := test.MakeTstatePath(t, dir)

	sts, err := ts.GetDir(dir)
	assert.Nil(t, err)

	log.Printf("%v: %v\n", dir, sp.Names(sts))

	// Your code here

	ts.Shutdown()
}

func TestExerciseS3(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Your code here

	ts.Shutdown()
}

func TestExerciseProc(t *testing.T) {
	ts := test.MakeTstateAll(t)

	p := proc.MakeProc("example", []string{})
	err := ts.Spawn(p)
	assert.Nil(t, err)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	// Once you modified cmd/user/example, you should
	// pass this test:
	assert.Equal(t, "Hello world", status.Msg())

	ts.Shutdown()
}
