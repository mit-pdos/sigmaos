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

func TestExample1(t *testing.T) {
	dir := sp.NAMED
	ts := test.MakeTstatePath(t, dir)

	sts, err := ts.GetDir(dir)
	assert.Nil(t, err)

	log.Printf("%v: %v\n", dir, sp.Names(sts))

	ts.Shutdown()
}
