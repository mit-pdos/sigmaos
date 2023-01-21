package realmclnt_test

import (
	"fmt"
	"log"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	// "sigmaos/perf"
	"sigmaos/fslib"
	"sigmaos/named"
	"sigmaos/proc"
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
	rc, err := realmclnt.MakeRealmClnt(ts.FsLib)
	assert.Nil(t, err)

	err = rc.MakeRealm(realm)
	assert.Nil(t, err)

	pn := path.Join(sp.REALMS, string(realm))
	sts, err := ts.GetDir(pn + "/")
	assert.Nil(t, err)

	log.Printf("names %v\n", sp.Names(sts))

	pclnt :=
		assert.True(t, fslib.Present(sts, named.InitDir), "initfs")

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
