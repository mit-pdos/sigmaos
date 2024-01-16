package kvgrp_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/groupmgr"
	"sigmaos/kvgrp"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	GRP    = "grp-0"
	N_REPL = 3
	N_KEYS = 10000
)

type Tstate struct {
	*test.Tstate
	grp string
	gm  *groupmgr.GroupMgr
	job string
}

func newTstate(t *testing.T, nrepl int, persist bool) *Tstate {
	ts := &Tstate{job: rand.String(4), grp: GRP}
	ts.Tstate = test.NewTstateAll(t)
	ts.MkDir(kvgrp.KVDIR, 0777)
	err := ts.MkDir(kvgrp.JobDir(ts.job), 0777)
	assert.Nil(t, err)
	mcfg := groupmgr.NewGroupConfig(nrepl, "kvd", []string{ts.grp, strconv.FormatBool(test.Overlays)}, 0, ts.job)
	if persist {
		mcfg.Persist(ts.SigmaClnt.FsLib)
	}
	ts.gm = mcfg.StartGrpMgr(ts.SigmaClnt, 0)
	cfg, err := kvgrp.WaitStarted(ts.SigmaClnt.FsLib, kvgrp.JobDir(ts.job), ts.grp)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	return ts
}

func (ts *Tstate) Shutdown(crash bool) {
	if crash {
		err := ts.gm.Crash()
		assert.Nil(ts.T, err)
	}
	ts.Tstate.Shutdown()
}

func TestCompile(t *testing.T) {
}

func TestStartStopRepl0(t *testing.T) {
	ts := newTstate(t, 0, false)

	sts, _, err := ts.ReadDir(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", sp.Names(sts), err)
	assert.Nil(t, err, "stat")

	_, err = ts.gm.StopGroup()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown(false)
}

func TestStartStopReplN(t *testing.T) {
	ts := newTstate(t, N_REPL, false)
	_, err := ts.gm.StopGroup()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown(false)
}

func (ts *Tstate) testRecover() {
	ts.Shutdown(true)
	time.Sleep(2 * fsetcd.LeaseTTL * time.Second)
	ts.Tstate = test.NewTstateAll(ts.T)
	gms, err := groupmgr.Recover(ts.SigmaClnt)
	assert.Nil(ts.T, err, "Recover")
	assert.Equal(ts.T, 1, len(gms))
	cfg, err := kvgrp.WaitStarted(ts.SigmaClnt.FsLib, kvgrp.JobDir(ts.job), ts.grp)
	assert.Nil(ts.T, err)
	time.Sleep(1 * fsetcd.LeaseTTL * time.Second)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	time.Sleep(1 * fsetcd.LeaseTTL * time.Second)
	gms[0].StopGroup()
	ts.RmDir(groupmgr.GRPMGRDIR)
	ts.Shutdown(false)
}

func TestRestartRepl0(t *testing.T) {
	ts := newTstate(t, 0, true)
	ts.testRecover()
}

func TestRestartReplN(t *testing.T) {
	ts := newTstate(t, N_REPL, true)
	ts.testRecover()
}
