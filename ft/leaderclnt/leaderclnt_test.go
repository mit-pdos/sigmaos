package leaderclnt_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/namesrv/clnt"
	"sigmaos/path"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
)

const (
	DIR string = "outdir"
)

func TestCompile(t *testing.T) {
}

func TestOldLeaderOK(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	l := leaderclnt.OldleaderTest(ts, sp.NAMED+DIR, "", sp.ROOTREALM)

	l.ReleaseLeadership()

	ts.Shutdown()
}

func TestOldLeaderCrash(t *testing.T) {
	const T = 1000
	crashpn := sp.NAMED + "crashnd.sem"

	e := crash.NewEventPath(crash.NAMED_CRASH, T, float64(1.0), crashpn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	nd1 := clnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := clnt.StartNamed(ts.SigmaClnt, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	// Start a new named
	nd2 := clnt.NewNamedProc(test.REALM1, ts.ProcEnv().UseDialProxy, false)
	db.DPrintf(db.TEST, "Starting a new named: %v", nd2.GetPid())
	if err := clnt.StartNamed(ts.SigmaClnt, nd2); !assert.Nil(ts.T, err, "Err startNamed 2: %v", err) {
		return
	}

	l := leaderclnt.OldleaderTest(ts, sp.NAMED+DIR, crashpn, test.REALM1)

	l.ReleaseLeadership()

	if err := clnt.StopNamed(ts.SigmaClnt, nd2); !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}

	ts.Shutdown()
}

func TestMemfs(t *testing.T) {
	dir := path.MarkResolve(filepath.Join(sp.MEMFS, sp.ANY))
	fencedir := filepath.Join(dir, sp.FENCEDIR)

	ts, err1 := test.NewTstatePath(t, dir)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	l := leaderclnt.OldleaderTest(ts, dir+DIR, "", sp.ROOTREALM)

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
