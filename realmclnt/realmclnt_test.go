package realmclnt_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	// "sigmaos/perf"
	"sigmaos/fslib"
	"sigmaos/named"
	"sigmaos/proc"
	"sigmaos/realmclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SLEEP_MSECS           = 2000
	REALM       sp.Trealm = "testrealm"
)

type Tstate struct {
	*test.Tstate
	rc *realmclnt.RealmClnt
	sc *sigmaclnt.SigmaClnt
}

func mkTstate(t *testing.T) *Tstate {
	ts := &Tstate{Tstate: test.MakeTstateAll(t)}

	rc, err := realmclnt.MakeRealmClnt(ts.FsLib)
	assert.Nil(t, err)
	ts.rc = rc

	err = rc.MakeRealm(REALM)
	assert.Nil(t, err)

	sc, err := sigmaclnt.MkSigmaClntRealm(ts.FsLib, "testrealm", REALM)
	assert.Nil(t, err)
	ts.sc = sc

	return ts
}

func TestBasic(t *testing.T) {
	ts := mkTstate(t)

	sts1, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	log.Printf("names sched %v\n", sp.Names(sts1))

	sts, err := ts.sc.GetDir(sp.NAMED)
	assert.Nil(t, err)

	log.Printf("realm named root %v\n", sp.Names(sts))

	assert.True(t, fslib.Present(sts, named.InitDir), "initfs")

	sts, err = ts.sc.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	log.Printf("realm names sched %v\n", sp.Names(sts))

	assert.True(t, sts1[0].Name == sts[0].Name)

	ts.Shutdown()
}

func TestWaitExitSimpleSingle(t *testing.T) {
	ts := mkTstate(t)

	a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err := ts.sc.Spawn(a)
	assert.Nil(t, err, "Spawn")
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts.sc.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong")

	ts.Shutdown()
}
