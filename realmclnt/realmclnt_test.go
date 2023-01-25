package realmclnt_test

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/named"
	"sigmaos/proc"
	"sigmaos/realmclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SLEEP_MSECS           = 2000
	REALM       sp.Trealm = "testrealm"
	N_ITER                = 2_500_000_000
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

	sc, err := sigmaclnt.MkSigmaClntRealmProc(ts.FsLib, "testrealm", REALM)
	assert.Nil(t, err)
	ts.sc = sc

	return ts
}

func calibrateCTimeLinux(ts *Tstate, nthread uint, niter int) time.Duration {
	if _, err := os.Stat("../bin/user/spinperf"); err != nil {
		db.DPrintf(db.ALWAYS, "Run make.sh --norace user to build linux spinperf binary")
		return 0
	}
	cmd := exec.Command("../bin/user/spinperf", []string{"false", strconv.Itoa(int(nthread)), strconv.Itoa(niter), "linux-baseline"}...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	start := time.Now()
	err := cmd.Start()
	assert.Nil(ts.T, err, "Err start: %v", err)
	err = cmd.Wait()
	assert.Nil(ts.T, err, "Err wait: %v", err)
	return time.Since(start)
}

func spawnSpinPerf(ts *Tstate, nthread uint, niter int, id string) proc.Tpid {
	p := proc.MakeProc("spinperf", []string{"true", strconv.Itoa(int(nthread)), strconv.Itoa(niter), id})
	err := ts.sc.Spawn(p)
	assert.Nil(ts.T, err, "Error spawn: %v", err)
	return p.GetPid()
}

func waitSpinPerf(ts *Tstate, pid proc.Tpid) time.Duration {
	status, err := ts.sc.WaitExit(pid)
	assert.Nil(ts.T, err, "WaitExit error")
	assert.True(ts.T, status.IsStatusOK(), "Exit status wrong")
	return time.Duration(status.Data().(float64))
}

func calibrateCTimeSigma(ts *Tstate, nthread uint, niter int) time.Duration {
	pid := spawnSpinPerf(ts, nthread, niter, "sigma-baseline")
	return waitSpinPerf(ts, pid)
}

func TestBasic(t *testing.T) {
	ts := mkTstate(t)

	db.DPrintf(db.TEST, "Local ip: %v", ts.sc.GetLocalIP())

	sts1, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	sts, err := ts.sc.GetDir(sp.NAMED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm named root %v\n", sp.Names(sts))

	assert.True(t, fslib.Present(sts, named.InitDir), "initfs")

	sts, err = ts.sc.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm names sched %v\n", sp.Names(sts))

	assert.True(t, sts1[0].Name == sts[0].Name)

	ts.Shutdown()
}

func TestWaitExitSimpleSingle(t *testing.T) {
	ts := mkTstate(t)

	sts1, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts.sc.GetLocalIP())

	a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts.sc.Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts.sc.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong")

	ts.Shutdown()
}

func TestSpinPerfCalibrate(t *testing.T) {
	ts := mkTstate(t)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	ctimeS := calibrateCTimeSigma(ts, linuxsched.NCores, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	db.DPrintf(db.TEST, "Calibrate Linux baseline")
	ctimeL := calibrateCTimeLinux(ts, linuxsched.NCores, N_ITER)
	db.DPrintf(db.TEST, "Linux baseline compute time: %v", ctimeL)

	ts.Shutdown()
}
