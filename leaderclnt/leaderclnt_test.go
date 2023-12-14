package leaderclnt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/leaderclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	dirnamed = sp.NAMED + "outdir"
)

func TestCompile(t *testing.T) {
}

func TestOldLeaderOK(t *testing.T) {
	ts := test.NewTstateAll(t)

	l := leaderclnt.OldleaderTest(ts, dirnamed, false)

	l.ReleaseLeadership()

	ts.Shutdown()
}

func TestOldLeaderCrash(t *testing.T) {
	ts := test.NewTstateAll(t)

	err := ts.Boot(sp.NAMEDREL)
	assert.Nil(t, err)

	l := leaderclnt.OldleaderTest(ts, dirnamed, true)

	l.ReleaseLeadership()

	ts.Shutdown()
}
