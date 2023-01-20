package realmclnt_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	// "sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/realmclnt"
	"sigmaos/test"
)

const (
	SLEEP_MSECS = 2000
)

func TestWaitExitSimpleSingle(t *testing.T) {
	ts := test.MakeTstateAll(t)

	rc, err := realmclnt.MakeRealmClnt(ts.FsLib)
	assert.Nil(t, err)

	err = rc.MakeRealm("testrealm")
	assert.Nil(t, err)

	a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong")

	ts.Shutdown()
}
