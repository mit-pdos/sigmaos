package named_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestBootNamed(t *testing.T) {
	ts := test.MakeTstateAll(t)

	sts, err1 := ts.GetDir(sp.NAMED + "/")
	assert.Nil(t, err1)
	log.Printf("named %v\n", sp.Names(sts))

	ts.Shutdown()
}
