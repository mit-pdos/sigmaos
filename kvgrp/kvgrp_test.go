package kvgrp_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
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

func makeTstate(t *testing.T, nrepl int, persist bool) *Tstate {
	ts := &Tstate{job: rand.String(4), grp: GRP}
	ts.Tstate = test.MakeTstateAll(t)
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

func (ts *Tstate) Shutdown() {
	ts.Tstate.Shutdown()
}

func TestStartStopRepl0(t *testing.T) {
	ts := makeTstate(t, 0, false)

	sts, _, err := ts.ReadDir(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", sp.Names(sts), err)
	assert.Nil(t, err, "stat")

	err = ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestStartStopRepl1(t *testing.T) {
	ts := makeTstate(t, 1, false)

	st, err := ts.Stat(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", st, err)
	assert.Nil(t, err, "stat")

	sts, _, err := ts.ReadDir(kvgrp.GrpPath(kvgrp.JobDir(ts.job), ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", sp.Names(sts), err)
	assert.Nil(t, err, "stat")

	err = ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestStartStopReplN(t *testing.T) {
	ts := makeTstate(t, N_REPL, false)
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestRestart(t *testing.T) {
	ts := makeTstate(t, 0, true)
	ts.Shutdown()
	time.Sleep(2 * sp.Conf.Session.TIMEOUT)
	ts.Tstate = test.MakeTstateAll(t)
	gms, err := groupmgr.Recover(ts.SigmaClnt)
	assert.Nil(ts.T, err, "Recover")
	assert.Equal(ts.T, 1, len(gms))
	cfg, err := kvgrp.WaitStarted(ts.SigmaClnt.FsLib, kvgrp.JobDir(ts.job), ts.grp)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	gms[0].Stop()
	ts.RmDir(kvgrp.KVDIR)
	ts.Shutdown()
}
