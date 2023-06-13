package realmclnt_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	proto "sigmaos/cache/proto"
	"sigmaos/cacheclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	rd "sigmaos/rand"
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

func calibrateCTimeLinux(ts *test.RealmTstate, nthread uint, niter int) time.Duration {
	// If spinperf bin has not been build, print an error message and return.
	if _, err := os.Stat("../bin/user/spinperf"); err != nil {
		db.DPrintf(db.ALWAYS, "Run make.sh --norace user to build linux spinperf binary")
		return 0
	}
	cmd := exec.Command("../bin/user/spinperf", []string{"false", strconv.Itoa(int(nthread)), strconv.Itoa(niter), "linux-baseline"}...)
	start := time.Now()
	err := cmd.Start()
	assert.Nil(ts.Ts.T, err, "Err start: %v", err)
	err = cmd.Wait()
	assert.Nil(ts.Ts.T, err, "Err wait: %v", err)
	return time.Since(start)
}

func spawnSpinPerf(ts *test.RealmTstate, ncore proc.Tcore, nthread uint, niter int, id string) proc.Tpid {
	p := proc.MakeProc("spinperf", []string{"true", strconv.Itoa(int(nthread)), strconv.Itoa(niter), id})
	p.SetNcore(ncore)
	err := ts.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Error spawn: %v", err)
	return p.GetPid()
}

func waitSpinPerf(ts *test.RealmTstate, pid proc.Tpid) time.Duration {
	status, err := ts.WaitExit(pid)
	assert.Nil(ts.Ts.T, err)
	assert.True(ts.Ts.T, status.IsStatusOK(), "Exit status wrong: %v", status)
	return time.Duration(status.Data().(float64))
}

func calibrateCTimeSigma(ts *test.RealmTstate, nthread uint, niter int) time.Duration {
	c := make(chan time.Duration)
	go runSpinPerf(ts, c, 0, nthread, niter, "sigma-baseline")
	return <-c
}

func runSpinPerf(ts *test.RealmTstate, c chan time.Duration, ncore proc.Tcore, nthread uint, niter int, id string) {
	pid := spawnSpinPerf(ts, ncore, nthread, niter, id)
	c <- waitSpinPerf(ts, pid)
}

func TestBasicSimple(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	db.DPrintf(db.TEST, "Local ip: %v", ts1.GetLocalIP())

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	sts, err := ts1.GetDir(sp.NAMED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm named root %v\n", sp.Names(sts))

	assert.True(t, fslib.Present(sts, []string{sp.UXREL}), "initfs")

	sts, err = ts1.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm names sched %v\n", sp.Names(sts))

	assert.True(t, sts1[0].Name == sts[0].Name)

	err = ts1.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestBasicMultiRealmSingleNode(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	ts2 := test.MakeRealmTstate(rootts, REALM2)

	db.DPrintf(db.TEST, "[%v] Local ip: %v", REALM1, ts1.GetLocalIP())
	db.DPrintf(db.TEST, "[%v] Local ip: %v", REALM2, ts2.GetLocalIP())

	schedds1, err := ts1.GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	// Only one schedd so far.
	assert.True(rootts.T, len(schedds1) == 1, "Wrong number schedds %v", schedds1)

	schedds2, err := ts2.GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	// Only one schedd so far.
	assert.True(rootts.T, len(schedds2) == 1, "Wrong number schedds %v", schedds2)

	for i := range schedds1 {
		assert.Equal(t, schedds1[i].Name, schedds2[i].Name)
	}

	rootts.Shutdown()
}

func TestBasicMultiRealmMultiNode(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rootts.BootNode(1)
	time.Sleep(2 * sp.Conf.Realm.RESIZE_INTERVAL)
	ts2 := test.MakeRealmTstate(rootts, REALM2)

	db.DPrintf(db.TEST, "[%v] named addr: %v", REALM1, ts1.NamedAddr())
	db.DPrintf(db.TEST, "[%v] named addr: %v", REALM2, ts2.NamedAddr())

	// Should have a public and private address
	if test.Overlays {
		assert.Equal(rootts.T, 2, len(ts1.NamedAddr()))
		assert.Equal(rootts.T, 2, len(ts1.NamedAddr()))
	}

	schedds1, err := ts1.GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	assert.True(rootts.T, len(schedds1) == 2, "Wrong number schedds %v", schedds1)

	schedds2, err := ts2.GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	assert.True(rootts.T, len(schedds2) == 2, "Wrong number schedds %v", schedds2)

	for i := range schedds1 {
		assert.Equal(t, schedds1[i].Name, schedds2[i].Name)
	}

	rootts.Shutdown()
}

func TestWaitExitSimpleSingle(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts1.GetLocalIP())

	a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts1.Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts1.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong: %v", status)

	rootts.Shutdown()
}

func TestEvictSingle(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts1.GetLocalIP())

	a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", 60000), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts1.Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitstart")
	err = ts1.WaitStart(a.GetPid())
	db.DPrintf(db.TEST, "Post waitstart")
	assert.Nil(t, err, "waitstart error")

	db.DPrintf(db.TEST, "Pre evict")
	err = ts1.Evict(a.GetPid())
	db.DPrintf(db.TEST, "Post evict")
	assert.Nil(t, err, "evict error")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts1.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)

	rootts.Shutdown()
}

func TestEvictMultiRealm(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	// Make a second realm
	test.MakeRealmTstate(rootts, REALM2)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts1.GetLocalIP())

	a := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", 60000), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts1.Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitstart")
	err = ts1.WaitStart(a.GetPid())
	db.DPrintf(db.TEST, "Post waitstart")
	assert.Nil(t, err, "waitstart error")

	db.DPrintf(db.TEST, "Pre evict")
	err = ts1.Evict(a.GetPid())
	db.DPrintf(db.TEST, "Post evict")
	assert.Nil(t, err, "evict error")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts1.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)

	rootts.Shutdown()
}

func spawnDirreader(r *test.RealmTstate, pn string) *proc.Status {
	a := proc.MakeProc("dirreader", []string{pn})
	err := r.Spawn(a)
	assert.Nil(r.Ts.T, err, "Error spawn: %v", err)
	err = r.WaitStart(a.GetPid())
	assert.Nil(r.Ts.T, err, "waitstart error")
	status, err := r.WaitExit(a.GetPid())
	assert.Nil(r.Ts.T, err, "WaitExit error")
	return status
}

func TestRealmNetIsolationOK(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	// Make a second realm
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	job := rd.String(16)
	cm, err := cacheclnt.MkCacheMgr(ts1.SigmaClnt, job, 1, 0, true, test.Overlays)
	assert.Nil(t, err)

	cc, err := cacheclnt.MkCacheClnt([]*fslib.FsLib{ts1.FsLib}, job)
	assert.Nil(t, err)

	err = cc.Put("hello", &proto.CacheString{Val: "hello"})
	assert.Nil(t, err)

	_, err = cacheclnt.MkCacheClnt([]*fslib.FsLib{rootts.FsLib}, job)
	assert.NotNil(t, err)

	mnt, err := ts1.ReadMount(cc.Server(0))
	assert.Nil(t, err)

	// Remove public port
	if len(mnt.Addr) > 1 {
		mnt.Addr = mnt.Addr[:1]
	}

	pn := path.Join(sp.NAMED, "srv")
	err = ts1.MountService(pn, mnt)
	assert.Nil(t, err)

	pn = pn + "/"

	status := spawnDirreader(ts1, pn)
	if test.Overlays {
		assert.True(t, status.IsStatusOK())
	} else {
		assert.True(t, status.IsStatusOK())
	}

	cm.Stop()

	rootts.Shutdown()
}

func TestRealmNetIsolationFail(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	// Make a second realm
	ts2 := test.MakeRealmTstate(rootts, REALM2)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	job := rd.String(16)
	cm, err := cacheclnt.MkCacheMgr(ts1.SigmaClnt, job, 1, 0, true, test.Overlays)
	assert.Nil(t, err)

	cc, err := cacheclnt.MkCacheClnt([]*fslib.FsLib{ts1.FsLib}, job)
	assert.Nil(t, err)

	err = cc.Put("hello", &proto.CacheString{Val: "hello"})
	assert.Nil(t, err)

	_, err = cacheclnt.MkCacheClnt([]*fslib.FsLib{rootts.FsLib}, job)
	assert.NotNil(t, err)

	mnt, err := ts1.ReadMount(cc.Server(0))
	assert.Nil(t, err)

	// Remove public port
	if len(mnt.Addr) > 1 {
		mnt.Addr = mnt.Addr[:1]
	}

	pn := path.Join(sp.NAMED, "srv")
	err = ts2.MountService(pn, mnt)
	assert.Nil(t, err)

	pn = pn + "/"

	status := spawnDirreader(ts2, pn)
	if test.Overlays {
		assert.True(t, status.IsStatusErr())
	} else {
		assert.True(t, status.IsStatusOK())
		log.Printf("status %v %v\n", status.Msg(), status.Data())
	}

	cm.Stop()

	rootts.Shutdown()
}

func TestSpinPerfCalibrate(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	// -1 for named
	ctimeS := calibrateCTimeSigma(ts1, linuxsched.NCores-1, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	db.DPrintf(db.TEST, "Calibrate Linux baseline")
	// -1 for named
	ctimeL := calibrateCTimeLinux(ts1, linuxsched.NCores-1, N_ITER)
	db.DPrintf(db.TEST, "Linux baseline compute time: %v", ctimeL)

	rootts.Shutdown()
}

// Calculate slowdown %
func slowdown(baseline, dur time.Duration) float64 {
	return float64(dur) / float64(baseline)
}

func targetTime(baseline time.Duration, tslowdown float64) time.Duration {
	return time.Duration(float64(baseline) * tslowdown)
}

func TestSpinPerfDoubleSlowdown(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	// - 2 to account for NAMED reserved cores
	ctimeS := calibrateCTimeSigma(ts1, linuxsched.NCores-2, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	c := make(chan time.Duration)
	go runSpinPerf(ts1, c, 0, linuxsched.NCores-2, N_ITER, "spin1")
	go runSpinPerf(ts1, c, 0, linuxsched.NCores-2, N_ITER, "spin2")

	d1 := <-c
	d2 := <-c

	// Calculate slowdown
	d1sd := slowdown(ctimeS, d1)
	d2sd := slowdown(ctimeS, d2)

	// Target slowdown (x)
	tsd := 1.70

	// Check that execution time matches target time.
	assert.True(rootts.T, d1sd > tsd, "Spin perf 1 not enough slowdown (%v): %v <= %v", d1sd, d1, targetTime(ctimeS, tsd))
	assert.True(rootts.T, d2sd > tsd, "Spin perf 2 not enough slowdown (%v): %v <= %v", d1sd, d2, targetTime(ctimeS, tsd))

	rootts.Shutdown()
}

func TestSpinPerfDoubleBEandLC(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	// - 2 to account for NAMED reserved cores
	ctimeS := calibrateCTimeSigma(ts1, linuxsched.NCores-2, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	beC := make(chan time.Duration)
	lcC := make(chan time.Duration)
	// - 2 to account for NAMED reserved cores
	go runSpinPerf(ts1, lcC, proc.Tcore(linuxsched.NCores-2), linuxsched.NCores-2, N_ITER, "lcspin")
	go runSpinPerf(ts1, beC, 0, linuxsched.NCores-2, N_ITER, "bespin")

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
	assert.True(rootts.T, lcSD <= lcMaxSD, "LC too much slowdown (%v): %v > %v", lcSD, durLC, targetTime(ctimeS, lcMaxSD))
	assert.True(rootts.T, beSD <= beMaxSD, "BE too much slowdown (%v): %v > %v", beSD, durBE, targetTime(ctimeS, beMaxSD))
	assert.True(rootts.T, beSD > beMinSD, "BE not enough slowdown (%v): %v < %v", beSD, durBE, targetTime(ctimeS, beMinSD))

	rootts.Shutdown()
}

func TestSpinPerfDoubleBEandLCMultiRealm(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	ts2 := test.MakeRealmTstate(rootts, REALM2)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	// - 2 to account for NAMED reserved cores
	ctimeS := calibrateCTimeSigma(ts1, linuxsched.NCores-2, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	beC := make(chan time.Duration)
	lcC := make(chan time.Duration)
	// - 2 to account for NAMED reserved cores
	go runSpinPerf(ts1, lcC, proc.Tcore(linuxsched.NCores-2), linuxsched.NCores-2, N_ITER, "lcspin")
	go runSpinPerf(ts2, beC, 0, linuxsched.NCores-2, N_ITER, "bespin")

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
	assert.True(rootts.T, lcSD <= lcMaxSD, "LC too much slowdown (%v): %v > %v", lcSD, durLC, targetTime(ctimeS, lcMaxSD))
	assert.True(rootts.T, beSD <= beMaxSD, "BE too much slowdown (%v): %v > %v", beSD, durBE, targetTime(ctimeS, beMaxSD))
	assert.True(rootts.T, beSD > beMinSD, "BE not enough slowdown (%v): %v < %v", beSD, durBE, targetTime(ctimeS, beMinSD))

	rootts.Shutdown()
}
