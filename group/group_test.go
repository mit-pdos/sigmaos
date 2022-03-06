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
	ts.gm = groupmgr.Start(ts.System.FsLib, ts.System.ProcClnt, 1, "bin/user/kvd", []string{GRP + "0"}, 0, 0)
	return ts
}

func TestStartStopGroup(t *testing.T) {
	ts := makeTstate(t, 0)
	err := ts.gm.Stop()
	assert.Nil(ts.T, err, "Stop")
	ts.Shutdown()
}
