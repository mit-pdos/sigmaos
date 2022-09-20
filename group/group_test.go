package group_test

import (
	"path"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/group"
	"sigmaos/groupmgr"
	np "sigmaos/ninep"
	"sigmaos/test"
)

const (
	CRASH_KVD = 5000
	GRP_PATH  = "name/group/grp-0"
	N_REPL    = 3
	N_KEYS    = 10000
	JOBDIR    = "name/"
)

type Tstate struct {
	*test.Tstate
	gm *groupmgr.GroupMgr
}

func makeTstate(t *testing.T, nrepl, ncrash int) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.gm = groupmgr.Start(ts.System.FsLib, ts.System.ProcClnt, nrepl, "user/kvd", []string{group.GRP + "0"}, JOBDIR, 0, ncrash, CRASH_KVD, 0, 0)
	return ts
}

func (ts *Tstate) setupKeys(nkeys int) {
	db.DPrintf("TEST", "setupKeys")
	for i := 0; i < nkeys; i++ {
		i_str := strconv.Itoa(i)
		fname := path.Join(GRP_PATH, i_str)
		_, err := ts.PutFile(fname, 0777, np.OWRITE|np.OREAD, []byte(i_str))
		assert.Nil(ts.T, err, "Put %v", err)
	}
	db.DPrintf("TEST", "done setupKeys")
}

func (ts *Tstate) testGetPutSet(nkeys int) {
	db.DPrintf("TEST", "testGetPutSet")
	for i := 0; i < nkeys; i++ {
		i_str := strconv.Itoa(i)
		fname := path.Join(GRP_PATH, i_str)
		b, err := ts.GetFile(fname)
		assert.Nil(ts.T, err, "Get %v", err)
		assert.Equal(ts.T, i_str, string(b), "Didn't read expected")
		_, err = ts.PutFile(fname, 0777, np.OWRITE|np.OREAD, []byte(i_str))
		assert.NotNil(ts.T, err, "Put nil")
		_, err = ts.SetFile(fname, []byte(i_str+i_str), np.OWRITE|np.OREAD, 0)
		assert.Nil(ts.T, err, "Set %v", err)
	}
	db.DPrintf("TEST", "done testGetPutSet")
}

func TestStartStop(t *testing.T) {
	ts := makeTstate(t, 0, 0)
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestStartStopRepl1(t *testing.T) {
	ts := makeTstate(t, 1, 0)
	err := ts.gm.Stop()
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
	ts.testGetPutSet(N_KEYS)
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

func TestGetPutSetFail1(t *testing.T) {
	ts := makeTstate(t, N_REPL, 1)
	ts.setupKeys(N_KEYS)
	ts.testGetPutSet(N_KEYS)
	db.DPrintf("TEST", "Pre stop")
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	db.DPrintf("TEST", "Post stop")
	ts.Shutdown()
}

//func TestStartStopGroup(t *testing.T) {
//	ts := makeTstate(t, 0)
//	err := ts.gm.Stop()
//	assert.Nil(ts.T, err, "Stop")
//	ts.Shutdown()
//}

// func follower(t *testing.T, i int, N int, fn string) {
// 	I := strconv.Itoa(i)
// 	fsl := fslib.MakeFsLibAddr("fsl"+I, fslib.Named())
// 	f := fenceclnt.MakeFenceClnt(fsl, fn, 0, []string{np.NAMED})
// 	for n := 0; n < N; {
// 		b, err := f.AcquireFenceR()
// 		assert.Nil(t, err, "AcquireFenceR")
// 		m, err := strconv.Atoi(string(b))
// 		assert.Nil(t, err, "Atoi")
// 		n = m
// 	}
// }

// func TestFollower(t *testing.T) {
// 	const N = 10
// 	const W = 10

// 	fn := "name/config"
// 	for i := 0; i < W; i++ {
// 		go follower(t, i, N, fn)
// 	}
// 	fsl := fslib.MakeFsLibAddr("fsl", fslib.Named())
// 	f := fenceclnt.MakeFenceClnt(fsl, fn, 0, []string{np.NAMED})
// 	for i := 0; i < N; i++ {
// 		err := f.AcquireFenceW([]byte(strconv.Itoa(i)))
// 		assert.Nil(t, err, "AcquireFenceW")
// 		err := f.Remove(fn)
// 	}
// }
