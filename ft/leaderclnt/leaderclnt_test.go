package leaderclnt_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
)

const (
	DIR = "outdir"
)

func TestCompile(t *testing.T) {
}

func TestOldLeaderOK(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	l := leaderclnt.OldleaderTest(ts, sp.NAMED+DIR, "")

	l.ReleaseLeadership()

	ts.Shutdown()
}

func TestOldLeaderCrash(t *testing.T) {
	const T = 1000
	fn := sp.NAMED + "crashnd.sem"

	e := crash.NewEventPath(crash.NAMED_CRASH, T, float64(1.0), fn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	err = ts.BootEnv(sp.NAMEDREL, []string{"SIGMAFAIL="})
	assert.Nil(t, err)

	l := leaderclnt.OldleaderTest(ts, sp.NAMED+DIR, fn)

	l.ReleaseLeadership()

	ts.Shutdown()
}

func TestMemfs(t *testing.T) {
	dir := sp.MEMFS + sp.ANY + "/"
	fencedir := filepath.Join(dir, sp.FENCEDIR)

	ts, err1 := test.NewTstatePath(t, dir)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	l := leaderclnt.OldleaderTest(ts, dir+DIR, "")

	sts, err := l.GetFences(fencedir)
	assert.Nil(ts.T, err, "GetFences")
	assert.Equal(ts.T, 1, len(sts), "Fences")

	db.DPrintf(db.TEST, "fences %v\n", sp.Names(sts))

	err = l.RemoveFence([]string{fencedir})
	assert.Nil(ts.T, err, "RemoveFences")

	sts, err = l.GetFences(fencedir)
	assert.Nil(ts.T, err, "GetFences")
	assert.Equal(ts.T, 0, len(sts), "Fences")

	db.DPrintf(db.TEST, "fences %v\n", sp.Names(sts))

	l.ReleaseLeadership()

	ts.Shutdown()
}
