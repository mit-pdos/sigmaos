package realmclnt_test

import (
	// "fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	// db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	// "sigmaos/perf"
	"sigmaos/fslib"
	"sigmaos/named"
	// "sigmaos/proc"
	"sigmaos/realmclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SLEEP_MSECS = 2000
)

func TestWaitExitSimpleSingle(t *testing.T) {
	ts := test.MakeTstateAll(t)
	realm := sp.Trealm("testrealm")

	sts1, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	log.Printf("names sched %v\n", sp.Names(sts1))

	rc, err := realmclnt.MakeRealmClnt(ts.FsLib)
	assert.Nil(t, err)

	err = rc.MakeRealm(realm)
	assert.Nil(t, err)

	sc, err := sigmaclnt.MkSigmaClntRealmProc(ts.FsLib, "testrealm", realm)
	assert.Nil(t, err)

	sts, err := sc.GetDir(sp.NAMED)
	assert.Nil(t, err)

	log.Printf("realm named root %v\n", sp.Names(sts))

	assert.True(t, fslib.Present(sts, named.InitDir), "initfs")

	sts, err = sc.GetDir(sp.SCHEDDREL + "/")
	assert.Nil(t, err)

	log.Printf("names realm sched %v\n", sp.Names(sts))

	assert.True(t, sts1[0].Name == sts[0].Name)

	// a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	// db.DPrintf(db.TEST, "Pre spawn")
	// err = sc.Spawn(a)
	// assert.Nil(t, err, "Spawn")
	// db.DPrintf(db.TEST, "Post spawn")

	// db.DPrintf(db.TEST, "Pre waitexit")
	// status, err := sc.WaitExit(a.GetPid())
	// db.DPrintf(db.TEST, "Post waitexit")
	// assert.Nil(t, err, "WaitExit error")
	// assert.True(t, status.IsStatusOK(), "Exit status wrong")

	ts.Shutdown()
}
