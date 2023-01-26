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
	// If spinperf bin has not been build, print an error message and return.
	if _, err := os.Stat("../bin/user/spinperf"); err != nil {
		db.DPrintf(db.ALWAYS, "Run make.sh --norace user to build linux spinperf binary")
		return 0
	}
	cmd := exec.Command("../bin/user/spinperf", []string{"false", strconv.Itoa(int(nthread)), strconv.Itoa(niter), "linux-baseline"}...)
	start := time.Now()
	err := cmd.Start()
	assert.Nil(ts.T, err, "Err start: %v", err)
	err = cmd.Wait()
	assert.Nil(ts.T, err, "Err wait: %v", err)
	return time.Since(start)
}

func spawnSpinPerf(ts *Tstate, ncore proc.Tcore, nthread uint, niter int, id string) proc.Tpid {
	p := proc.MakeProc("spinperf", []string{"true", strconv.Itoa(int(nthread)), strconv.Itoa(niter), id})
	p.SetNcore(ncore)
	err := ts.sc.Spawn(p)
	assert.Nil(ts.T, err, "Error spawn: %v", err)
	return p.GetPid()
}

func waitSpinPerf(ts *Tstate, pid proc.Tpid) time.Duration {
	status, err := ts.sc.WaitExit(pid)
	assert.Nil(ts.T, err, "WaitExit error")
	assert.True(ts.T, status.IsStatusOK(), "Exit status wrong: %v", status)
	return time.Duration(status.Data().(float64))
}

func calibrateCTimeSigma(ts *Tstate, nthread uint, niter int) time.Duration {
	c := make(chan time.Duration)
	go runSpinPerf(ts, c, 0, nthread, niter, "sigma-baseline")
	return <-c
}

func runSpinPerf(ts *Tstate, c chan time.Duration, ncore proc.Tcore, nthread uint, niter int, id string) {
	pid := spawnSpinPerf(ts, ncore, nthread, niter, id)
	c <- waitSpinPerf(ts, pid)
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
	assert.True(t, status.IsStatusOK(), "Exit status wrong: %v", status)

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

// Calculate slowdown %
func slowdown(baseline, dur time.Duration) float64 {
	return float64(dur) / float64(baseline)
}

func targetTime(baseline time.Duration, tslowdown float64) time.Duration {
	return time.Duration(float64(baseline) * tslowdown)
}

func TestSpinPerfDoubleSlowdown(t *testing.T) {
	ts := mkTstate(t)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	ctimeS := calibrateCTimeSigma(ts, linuxsched.NCores, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	c := make(chan time.Duration)
	go runSpinPerf(ts, c, 0, linuxsched.NCores, N_ITER, "spin1")
	go runSpinPerf(ts, c, 0, linuxsched.NCores, N_ITER, "spin2")

	d1 := <-c
	d2 := <-c

	// Calculate slowdown
	d1sd := slowdown(ctimeS, d1)
	d2sd := slowdown(ctimeS, d2)

	// Target slowdown (x)
	tsd := 1.80

	// Check that execution time matches target time.
	assert.True(ts.T, d1sd > tsd, "Spin perf 1 not enough slowdown (%v): %v <= %v", d1sd, d1, targetTime(ctimeS, tsd))
	assert.True(ts.T, d2sd > tsd, "Spin perf 2 not enough slowdown (%v): %v <= %v", d1sd, d2, targetTime(ctimeS, tsd))

	ts.Shutdown()
}

func TestSpinPerfDoubleBEandLC(t *testing.T) {
	ts := mkTstate(t)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	ctimeS := calibrateCTimeSigma(ts, linuxsched.NCores, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	beC := make(chan time.Duration)
	lcC := make(chan time.Duration)
	go runSpinPerf(ts, lcC, proc.Tcore(linuxsched.NCores), linuxsched.NCores, N_ITER, "lcspin")
	go runSpinPerf(ts, beC, 0, linuxsched.NCores, N_ITER, "bespin")

	durBE := <-beC
	durLC := <-lcC

	// Calculate slodown
	beSD := slowdown(ctimeS, durBE)
	lcSD := slowdown(ctimeS, durLC)

	// Target slowdown (x)
	beMinSD := 1.5
	beMaxSD := 2.5
	lcMaxSD := 1.1

	// Check that execution time matches target time.
	assert.True(ts.T, lcSD <= lcMaxSD, "LC too much slowdown (%v): %v > %v", lcSD, targetTime(ctimeS, lcMaxSD))
	assert.True(ts.T, beSD <= beMaxSD, "BE too much slowdown (%v): %v > %v", beSD, targetTime(ctimeS, beMaxSD))
	assert.True(ts.T, beSD > beMinSD, "BE not enough slowdown (%v): %v < %v", beSD, targetTime(ctimeS, beMinSD))

	ts.Shutdown()
}
