package realmclnt_test

import (
	"fmt"
	"net"
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
	SLEEP_MSECS = 2000
	N_ITER      = 5_000_000_000

	// Realms
	REALM1 sp.Trealm = "testrealm"
	REALM2 sp.Trealm = "testrealm2"
)

type Tstate struct {
	*test.Tstate
	rc  *realmclnt.RealmClnt
	scs map[sp.Trealm]*sigmaclnt.SigmaClnt
}

func mkTstate(t *testing.T) *Tstate {
	ts := &Tstate{
		Tstate: test.MakeTstateRealm(t),
		scs:    make(map[sp.Trealm]*sigmaclnt.SigmaClnt),
	}

	rc, err := realmclnt.MakeRealmClnt(ts.FsLib)
	assert.Nil(t, err)
	ts.rc = rc

	ts.mkRealm(REALM1)

	return ts
}

func (ts *Tstate) mkRealm(realm sp.Trealm) {
	err := ts.rc.MakeRealm(realm)
	assert.Nil(ts.T, err)

	sc, err := sigmaclnt.MkSigmaClntRealmProc(ts.FsLib, "test"+realm.String(), realm)
	assert.Nil(ts.T, err)
	ts.scs[realm] = sc
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

func spawnSpinPerf(ts *Tstate, realm sp.Trealm, ncore proc.Tcore, nthread uint, niter int, id string) proc.Tpid {
	p := proc.MakeProc("spinperf", []string{"true", strconv.Itoa(int(nthread)), strconv.Itoa(niter), id})
	p.SetNcore(ncore)
	err := ts.scs[realm].Spawn(p)
	assert.Nil(ts.T, err, "Error spawn: %v", err)
	return p.GetPid()
}

func waitSpinPerf(ts *Tstate, realm sp.Trealm, pid proc.Tpid) time.Duration {
	status, err := ts.scs[realm].WaitExit(pid)
	assert.Nil(ts.T, err, "WaitExit error")
	assert.True(ts.T, status.IsStatusOK(), "Exit status wrong: %v", status)
	return time.Duration(status.Data().(float64))
}

func calibrateCTimeSigma(ts *Tstate, realm sp.Trealm, nthread uint, niter int) time.Duration {
	c := make(chan time.Duration)
	go runSpinPerf(ts, realm, c, 0, nthread, niter, "sigma-baseline")
	return <-c
}

func runSpinPerf(ts *Tstate, realm sp.Trealm, c chan time.Duration, ncore proc.Tcore, nthread uint, niter int, id string) {
	pid := spawnSpinPerf(ts, realm, ncore, nthread, niter, id)
	c <- waitSpinPerf(ts, realm, pid)
}

func TestBasic(t *testing.T) {
	ts := mkTstate(t)

	db.DPrintf(db.TEST, "Local ip: %v", ts.scs[REALM1].GetLocalIP())

	sts1, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	sts, err := ts.scs[REALM1].GetDir(sp.NAMED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm named root %v\n", sp.Names(sts))

	assert.True(t, fslib.Present(sts, named.InitDir), "initfs")

	sts, err = ts.scs[REALM1].GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm names sched %v\n", sp.Names(sts))

	assert.True(t, sts1[0].Name == sts[0].Name)

	ts.Shutdown()
}

func TestBasicMultiRealmSingleNode(t *testing.T) {
	ts := mkTstate(t)
	ts.mkRealm(REALM2)

	db.DPrintf(db.TEST, "[%v] Local ip: %v", REALM1, ts.scs[REALM1].GetLocalIP())
	db.DPrintf(db.TEST, "[%v] Local ip: %v", REALM2, ts.scs[REALM2].GetLocalIP())

	schedds1, err := ts.scs[REALM1].GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	// Only one schedd so far.
	assert.True(ts.T, len(schedds1) == 1, "Wrong number schedds %v", schedds1)

	schedds2, err := ts.scs[REALM2].GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	// Only one schedd so far.
	assert.True(ts.T, len(schedds2) == 1, "Wrong number schedds %v", schedds2)

	for i := range schedds1 {
		assert.Equal(t, schedds1[i].Name, schedds2[i].Name)
	}

	ts.Shutdown()
}

func TestBasicMultiRealmMultiNode(t *testing.T) {
	ts := mkTstate(t)
	ts.BootNode(1)
	time.Sleep(2 * sp.Conf.Realm.RESIZE_INTERVAL)
	ts.mkRealm(REALM2)

	db.DPrintf(db.TEST, "[%v] named addr: %v", REALM1, ts.scs[REALM1].NamedAddr())
	db.DPrintf(db.TEST, "[%v] named addr: %v", REALM2, ts.scs[REALM2].NamedAddr())

	nd1ip, _, err := net.SplitHostPort(ts.scs[REALM1].NamedAddr()[0])
	assert.Nil(ts.T, err)
	nd2ip, _, err := net.SplitHostPort(ts.scs[REALM2].NamedAddr()[0])
	assert.Nil(ts.T, err)

	assert.NotEqual(ts.T, nd1ip, nd2ip, "Nameds were spawned to the same node")

	schedds1, err := ts.scs[REALM1].GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	// Only one schedd so far.
	assert.True(ts.T, len(schedds1) == 2, "Wrong number schedds %v", schedds1)

	schedds2, err := ts.scs[REALM2].GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	// Only one schedd so far.
	assert.True(ts.T, len(schedds2) == 2, "Wrong number schedds %v", schedds2)

	for i := range schedds1 {
		assert.Equal(t, schedds1[i].Name, schedds2[i].Name)
	}

	ts.Shutdown()
}

func TestWaitExitSimpleSingle(t *testing.T) {
	ts := mkTstate(t)

	sts1, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts.scs[REALM1].GetLocalIP())

	a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts.scs[REALM1].Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts.scs[REALM1].WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong: %v", status)

	ts.Shutdown()
}

func TestSpinPerfCalibrate(t *testing.T) {
	ts := mkTstate(t)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	ctimeS := calibrateCTimeSigma(ts, REALM1, linuxsched.NCores, N_ITER)
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
	ctimeS := calibrateCTimeSigma(ts, REALM1, linuxsched.NCores, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	c := make(chan time.Duration)
	go runSpinPerf(ts, REALM1, c, 0, linuxsched.NCores, N_ITER, "spin1")
	go runSpinPerf(ts, REALM1, c, 0, linuxsched.NCores, N_ITER, "spin2")

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
	ctimeS := calibrateCTimeSigma(ts, REALM1, linuxsched.NCores-1, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	beC := make(chan time.Duration)
	lcC := make(chan time.Duration)
	go runSpinPerf(ts, REALM1, lcC, proc.Tcore(linuxsched.NCores-1), linuxsched.NCores-1, N_ITER, "lcspin")
	go runSpinPerf(ts, REALM1, beC, 0, linuxsched.NCores-1, N_ITER, "bespin")

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
	assert.True(ts.T, lcSD <= lcMaxSD, "LC too much slowdown (%v): %v > %v", lcSD, durLC, targetTime(ctimeS, lcMaxSD))
	assert.True(ts.T, beSD <= beMaxSD, "BE too much slowdown (%v): %v > %v", beSD, durBE, targetTime(ctimeS, beMaxSD))
	assert.True(ts.T, beSD > beMinSD, "BE not enough slowdown (%v): %v < %v", beSD, durBE, targetTime(ctimeS, beMinSD))

	ts.Shutdown()
}

func TestSpinPerfDoubleBEandLCMultiRealm(t *testing.T) {
	ts := mkTstate(t)

	ts.mkRealm(REALM2)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	ctimeS := calibrateCTimeSigma(ts, REALM1, linuxsched.NCores-1, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	beC := make(chan time.Duration)
	lcC := make(chan time.Duration)
	go runSpinPerf(ts, REALM1, lcC, proc.Tcore(linuxsched.NCores-1), linuxsched.NCores-1, N_ITER, "lcspin")
	go runSpinPerf(ts, REALM2, beC, 0, linuxsched.NCores-1, N_ITER, "bespin")

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
	assert.True(ts.T, lcSD <= lcMaxSD, "LC too much slowdown (%v): %v > %v", lcSD, durLC, targetTime(ctimeS, lcMaxSD))
	assert.True(ts.T, beSD <= beMaxSD, "BE too much slowdown (%v): %v > %v", beSD, durBE, targetTime(ctimeS, beMaxSD))
	assert.True(ts.T, beSD > beMinSD, "BE not enough slowdown (%v): %v < %v", beSD, durBE, targetTime(ctimeS, beMinSD))

	ts.Shutdown()
}
