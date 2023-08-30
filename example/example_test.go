package example_test

import (
	// Go imports:
	"log"
	"testing"

	// External imports:
	"github.com/stretchr/testify/assert"

	// SigmaOS imports:
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
