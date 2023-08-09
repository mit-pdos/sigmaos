package kvgrp_test

import (
	"path"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/cache"
	"sigmaos/cache/proto"
	"sigmaos/cacheclnt"
	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/groupmgr"
	"sigmaos/kvgrp"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	GRP       = "grp-0"
	CRASH_KVD = 5000
	N_REPL    = 3
	N_KEYS    = 10000
	JOBDIR    = "name/group"
)

type Tstate struct {
	*test.Tstate
	grp string
	gm  *groupmgr.GroupMgr
	cc  *cacheclnt.CacheClnt
}

func makeTstate(t *testing.T, nrepl, ncrash int) *Tstate {
	ts := &Tstate{grp: GRP}
	ts.Tstate = test.MakeTstateAll(t)
	ts.RmDir(JOBDIR)
	ts.MkDir(JOBDIR, 0777)
	ts.gm = groupmgr.Start(ts.SigmaClnt, nrepl, "kvd", []string{ts.grp, strconv.FormatBool(test.Overlays)}, JOBDIR, 0, ncrash, CRASH_KVD, 0, 0)
	cfg, err := kvgrp.WaitStarted(ts.SigmaClnt.FsLib, JOBDIR, ts.grp)
	assert.Nil(t, err)
	ts.cc = cacheclnt.NewCacheClnt([]*fslib.FsLib{ts.SigmaClnt.FsLib}, JOBDIR, 1)
	db.DPrintf(db.TEST, "cfg %v\n", cfg)
	return ts
}

func (ts *Tstate) Shutdown() {
	ts.Tstate.Shutdown()
}

func (ts *Tstate) setupKeys(nkeys int) {
	db.DPrintf(db.TEST, "setupKeys")
	srv := path.Join(kvgrp.GrpPath(JOBDIR, ts.grp))
	err := ts.cc.CreateShard(srv, cache.Tshard(0), sp.NullFence(), make(cachesrv.Tcache))
	assert.Nil(ts.T, err, "CreateShard %v", err)
	for i := 0; i < nkeys; i++ {
		i_str := strconv.Itoa(i)
		err := ts.cc.PutSrv(srv, i_str, &proto.CacheString{Val: i_str})
		assert.Nil(ts.T, err, "Put %v", err)
	}
	db.DPrintf(db.TEST, "done setupKeys")
}

func (ts *Tstate) testGetPut(nkeys int) {
	db.DPrintf(db.TEST, "testGetPutSet")
	for i := 0; i < nkeys; i++ {
		i_str := strconv.Itoa(i)
		srv := path.Join(kvgrp.GrpPath(JOBDIR, ts.grp))
		res := &proto.CacheString{}
		err := ts.cc.GetSrv(srv, i_str, res)
		assert.Nil(ts.T, err, "GetSrv %v", err)
		assert.Equal(ts.T, i_str, res.Val, "Didn't read expected")
		err = ts.cc.PutSrv(srv, i_str, &proto.CacheString{Val: i_str + i_str})
		assert.Nil(ts.T, err, "PutSrv")
	}
	db.DPrintf(db.TEST, "done testGetPutSet")
}

func TestStartStopRepl0(t *testing.T) {
	ts := makeTstate(t, 0, 0)

	sts, _, err := ts.ReadDir(kvgrp.GrpPath(JOBDIR, ts.grp) + "/")
	db.DPrintf(db.TEST, "Stat: %v %v\n", sp.Names(sts), err)
	assert.Nil(t, err, "stat")

	err = ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestStartStopRepl1(t *testing.T) {
	ts := makeTstate(t, 1, 0)

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
	ts := makeTstate(t, N_REPL, 0)
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestGetPutSetReplOK(t *testing.T) {
	ts := makeTstate(t, N_REPL, 0)
	ts.setupKeys(N_KEYS)
	ts.testGetPut(N_KEYS)
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestGetPutSetFail1(t *testing.T) {
	ts := makeTstate(t, N_REPL, 1)
	ts.setupKeys(N_KEYS)
	ts.testGetPut(N_KEYS)
	db.DPrintf(db.TEST, "Pre stop")
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	db.DPrintf(db.TEST, "Post stop")
	ts.Shutdown()
}
