package group

import (
	//"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/groupmgr"
	"ulambda/test"
)

type Tstate struct {
	*test.Tstate
	gm *groupmgr.GroupMgr
}

func makeTstate(t *testing.T, crash int) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.gm = groupmgr.Start(ts.System.FsLib, ts.System.ProcClnt, 1, "bin/user/kvd", []string{GRP + "0"}, 0, 0, 0, 0)
	return ts
}

func TestStartStopGroup(t *testing.T) {
	ts := makeTstate(t, 0)
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}

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
