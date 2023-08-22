package kvgrp_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/cacheclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/groupmgr"
	"sigmaos/kvgrp"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	GRP    = "grp-0"
	N_REPL = 3
	N_KEYS = 10000
	JOBDIR = "name/group"
)

type Tstate struct {
	*test.Tstate
	grp string
	gm  *groupmgr.GroupMgr
	cc  *cacheclnt.CacheClnt
}

func makeTstate(t *testing.T, nrepl int) *Tstate {
	ts := &Tstate{grp: GRP}
	ts.Tstate = test.MakeTstateAll(t)
	ts.RmDir(JOBDIR)
	ts.MkDir(JOBDIR, 0777)
	mcfg := groupmgr.NewGroupConfig(ts.SigmaClnt, nrepl, "kvd", []string{ts.grp, strconv.FormatBool(test.Overlays)}, 0, JOBDIR)
	ts.gm = mcfg.Start(0)
	cfg, err := kvgrp.WaitStarted(ts.SigmaClnt.FsLib, JOBDIR, ts.grp)
	assert.Nil(t, err)
	ts.cc = cacheclnt.NewCacheClnt([]*fslib.FsLib{ts.SigmaClnt.FsLib}, JOBDIR, 1)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	return ts
}

func (ts *Tstate) Shutdown() {
	ts.Tstate.Shutdown()
}

func TestStartStopRepl0(t *testing.T) {
	ts := makeTstate(t, 0)

	sts, _, err := ts.ReadDir(kvgrp.GrpPath(JOBDIR, ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", sp.Names(sts), err)
	assert.Nil(t, err, "stat")

	err = ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestStartStopRepl1(t *testing.T) {
	ts := makeTstate(t, 1)

	st, err := ts.Stat(kvgrp.GrpPath(JOBDIR, ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", st, err)
	assert.Nil(t, err, "stat")

	sts, _, err := ts.ReadDir(kvgrp.GrpPath(JOBDIR, ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", sp.Names(sts), err)
	assert.Nil(t, err, "stat")

	err = ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestStartStopReplN(t *testing.T) {
	ts := makeTstate(t, N_REPL)
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}
