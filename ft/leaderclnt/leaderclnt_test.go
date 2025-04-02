package leaderclnt_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
)

const (
	DIR  string     = "outdir"
	MCPU proc.Tmcpu = 1000
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

func newNamedProc(mcpu proc.Tmcpu, realm sp.Trealm, dialproxy bool, canFail bool) *proc.Proc {
	p := proc.NewProc(sp.NAMEDREL, []string{realm.String()})
	p.SetMcpu(mcpu)
	p.SetRealmSwitch(realm)
	p.GetProcEnv().UseDialProxy = dialproxy
	if !canFail {
		p.AppendEnv("SIGMAFAIL", "")
	} else {
		p.AppendEnv(proc.SIGMAFAIL, proc.GetSigmaFail())
	}
	return p
}

func startNamed(ts *test.Tstate, nd *proc.Proc) error {
	// Spawn the named proc
	if err := ts.Spawn(nd); !assert.Nil(ts.T, err, "Err spawn named: %v", err) {
		return err
	}
	db.DPrintf(db.TEST, "New named spawned: %v", nd.GetPid())

	// Wait for the proc to start
	if err := ts.WaitStart(nd.GetPid()); !assert.Nil(ts.T, err, "Err WaitStart named: %v", err) {
		return err
	}
	db.DPrintf(db.TEST, "New named started")

	// Wait for the named to start up
	_, err := ts.GetFileWatch(filepath.Join(sp.REALMS, test.REALM1.String()))
	// wait until the named has registered its endpoint and is ready to serve
	if !assert.Nil(ts.T, err, "Err GetFileWatch: %v", err) {
		return err
	}
	db.DPrintf(db.TEST, "New named ready to serve")
	return nil
}

func stopNamed(ts *test.Tstate, nd *proc.Proc) error {
	// Evict the new named
	err := ts.Evict(nd.GetPid())
	assert.Nil(ts.T, err, "Err evict named: %v", err)
	status, err := ts.WaitExit(nd.GetPid())
	if assert.Nil(ts.T, err, "Err WaitExit named: %v", err) {
		assert.True(ts.T, status.IsStatusEvicted(), "Wrong exit status: %v", status)
	}
	// Make sure the named EP has been removed
	ts.Remove(filepath.Join(sp.REALMS, test.REALM1.String()))
	return err
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

	nd1 := newNamedProc(MCPU, test.REALM1, ts.ProcEnv().UseDialProxy, true)
	if err := startNamed(ts, nd1); !assert.Nil(ts.T, err, "Err startNamed: %v", err) {
		return
	}

	// Start a new named
	nd2 := newNamedProc(MCPU, test.REALM1, ts.ProcEnv().UseDialProxy, false)
	db.DPrintf(db.TEST, "Starting a new named: %v", nd2.GetPid())
	if err := startNamed(ts, nd2); !assert.Nil(ts.T, err, "Err startNamed 2: %v", err) {
		return
	}

	l := leaderclnt.OldleaderTest(ts, sp.NAMED+DIR, fn, test.REALM1)

	l.ReleaseLeadership()

	if err := stopNamed(ts, nd2); !assert.Nil(ts.T, err, "Err stop named: %v", err) {
		return
	}

	ts.Shutdown()
}

func TestMemfs(t *testing.T) {
	dir := sp.MEMFS + sp.ANY + "/"
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
