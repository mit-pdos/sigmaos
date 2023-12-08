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
	"sigmaos/cachedsvc"
	"sigmaos/cachedsvcclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/proc"
	rd "sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SLEEP_MSECS = 2000
	N_ITER      = 5_000_000_000

	// Realms
	REALM1 sp.Trealm = "testrealm1"
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

func spawnSpinPerf(ts *test.RealmTstate, mcpu proc.Tmcpu, nthread uint, niter int, id string) sp.Tpid {
	p := proc.NewProc("spinperf", []string{"true", strconv.Itoa(int(nthread)), strconv.Itoa(niter), id})
	p.SetMcpu(mcpu)
	err := ts.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Error spawn: %v", err)
	return p.GetPid()
}

func waitSpinPerf(ts *test.RealmTstate, pid sp.Tpid) time.Duration {
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

func runSpinPerf(ts *test.RealmTstate, c chan time.Duration, mcpu proc.Tmcpu, nthread uint, niter int, id string) {
	pid := spawnSpinPerf(ts, mcpu, nthread, niter, id)
	c <- waitSpinPerf(ts, pid)
}

func TestBasicSimple(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)

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
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)
	ts2 := test.NewRealmTstate(rootts, REALM2)

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

	err = ts1.Remove()
	assert.Nil(t, err)
	err = ts2.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestBasicMultiRealmMultiNode(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)
	rootts.BootNode(1)
	time.Sleep(2 * sp.Conf.Realm.KERNEL_SRV_REFRESH_INTERVAL)
	ts2 := test.NewRealmTstate(rootts, REALM2)

	db.DPrintf(db.TEST, "[%v] named addr: %v", REALM1, ts1.GetNamedMount())
	db.DPrintf(db.TEST, "[%v] named addr: %v", REALM2, ts2.GetNamedMount())

	// Should have a public and private address
	if test.Overlays {
		assert.Equal(rootts.T, 2, len(ts1.GetNamedMount().Addr))
		assert.Equal(rootts.T, 2, len(ts1.GetNamedMount().Addr))
	}

	schedds1, err := ts1.GetDir(sp.SCHEDD)
	assert.Nil(t, err, "ErrGetDir SCHEDD: %v", err)
	assert.True(rootts.T, len(schedds1) == 2, "Wrong number schedds %v", schedds1)

	schedds2, err := ts2.GetDir(sp.SCHEDD)
	assert.Nil(t, err, "ErrGetDir SCHEDD: %v", err)
	assert.True(rootts.T, len(schedds2) == 2, "Wrong number schedds %v", schedds2)

	for i := range schedds1 {
		assert.Equal(t, schedds1[i].Name, schedds2[i].Name)
	}

	err = ts1.Remove()
	assert.Nil(t, err)
	err = ts2.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestBasicFairness(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)
	time.Sleep(2 * sp.Conf.Realm.KERNEL_SRV_REFRESH_INTERVAL)

	p1 := proc.NewProc("sleeper", []string{"100000s", "name/"})
	p1.SetMem(mem.GetTotalMem()/2 + 1)

	db.DPrintf(db.TEST, "Spawn big realm's proc")
	err := ts1.Spawn(p1)
	assert.Nil(rootts.T, err, "Err spawn: %v", err)
	err = ts1.WaitStart(p1.GetPid())
	assert.Nil(rootts.T, err, "Err WaitStart: %v", err)
	db.DPrintf(db.TEST, "Big realm's proc started")

	ts2 := test.NewRealmTstate(rootts, REALM2)
	db.DPrintf(db.TEST, "Created realm 2")

	p2 := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	p2.SetMem(mem.GetTotalMem())

	db.DPrintf(db.TEST, "Spawn small realm's proc")
	err = ts2.Spawn(p2)
	assert.Nil(rootts.T, err, "Err spawn: %v", err)
	err = ts2.WaitStart(p2.GetPid())
	assert.Nil(rootts.T, err, "Err WaitStart: %v", err)
	db.DPrintf(db.TEST, "Small realm's proc started")

	status, err := ts1.WaitExit(p1.GetPid())
	assert.Nil(rootts.T, err, "Err WaitExit: %v", err)
	assert.True(rootts.T, status.IsStatusEvicted(), "Wrong status: %v", status)

	status, err = ts2.WaitExit(p2.GetPid())
	assert.Nil(rootts.T, err, "Err WaitExit: %v", err)
	assert.True(rootts.T, status.IsStatusOK(), "Wrong status: %v", status)

	rootts.Shutdown()
}

func TestWaitExitSimpleSingle(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts1.GetLocalIP())

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts1.Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts1.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong: %v", status)

	err = ts1.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestEvictSingle(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts1.GetLocalIP())

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", 60000), "name/"})
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

	err = ts1.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestEvictMultiRealm(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	// Make a second realm
	ts2 := test.NewRealmTstate(rootts, REALM2)
	ts1 := test.NewRealmTstate(rootts, REALM1)

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts1.GetLocalIP())

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", 60000), "name/"})
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

	err = ts1.Remove()
	assert.Nil(t, err)
	err = ts2.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func spawnDirreader(r *test.RealmTstate, pn string) *proc.Status {
	a := proc.NewProc("dirreader", []string{pn})
	err := r.Spawn(a)
	assert.Nil(r.Ts.T, err, "Error spawn: %v", err)
	err = r.WaitStart(a.GetPid())
	assert.Nil(r.Ts.T, err, "waitstart error")
	status, err := r.WaitExit(a.GetPid())
	assert.Nil(r.Ts.T, err, "WaitExit error")
	return status
}

func TestRealmNetIsolationOK(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	// Make a second realm
	ts1 := test.NewRealmTstate(rootts, REALM1)

	job := rd.String(16)
	cm, err := cachedsvc.NewCacheMgr(ts1.SigmaClnt, job, 1, 0, true, test.Overlays)
	assert.Nil(t, err)

	cc, err := cachedsvcclnt.NewCachedSvcClnt([]*fslib.FsLib{ts1.FsLib}, job)
	assert.Nil(t, err)

	err = cc.Put("hello", &proto.CacheString{Val: "hello"})
	assert.Nil(t, err)

	_, err = cachedsvcclnt.NewCachedSvcClnt([]*fslib.FsLib{rootts.FsLib}, job)
	assert.NotNil(t, err)

	mnt, err := ts1.ReadMount(cc.Server(0))
	assert.Nil(t, err)

	// Remove public port
	if len(mnt.Addr) > 1 {
		mnt.Addr = mnt.Addr[:1]
	}

	pn := path.Join(sp.NAMED, "srv")
	err = ts1.MountService(pn, mnt, sp.NoLeaseId)
	assert.Nil(t, err)

	pn = pn + "/"

	status := spawnDirreader(ts1, pn)
	assert.True(t, status.IsStatusOK())

	cm.Stop()

	err = ts1.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestRealmNetIsolationFail(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	// Make a second realm
	ts2 := test.NewRealmTstate(rootts, REALM2)
	ts1 := test.NewRealmTstate(rootts, REALM1)

	job := rd.String(16)
	cm, err := cachedsvc.NewCacheMgr(ts1.SigmaClnt, job, 1, 0, true, test.Overlays)
	assert.Nil(t, err)

	cc, err := cachedsvcclnt.NewCachedSvcClnt([]*fslib.FsLib{ts1.FsLib}, job)
	assert.Nil(t, err)

	err = cc.Put("hello", &proto.CacheString{Val: "hello"})
	assert.Nil(t, err)

	_, err = cachedsvcclnt.NewCachedSvcClnt([]*fslib.FsLib{rootts.FsLib}, job)
	assert.NotNil(t, err)

	mnt, err := ts1.ReadMount(cc.Server(0))
	assert.Nil(t, err)

	// Remove public port
	if len(mnt.Addr) > 1 {
		mnt.Addr = mnt.Addr[:1]
	}

	pn := path.Join(sp.NAMED, "srv")
	err = ts2.MountService(pn, mnt, sp.NoLeaseId)
	assert.Nil(t, err)

	pn = pn + "/"

	status := spawnDirreader(ts2, pn)
	if test.Overlays {
		assert.True(t, status.IsStatusErr(), "Status is: %v", status)
	} else {
		assert.True(t, status.IsStatusOK())
		log.Printf("status %v %v\n", status.Msg(), status.Data())
	}

	cm.Stop()

	err = ts1.Remove()
	assert.Nil(t, err)
	err = ts2.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestSpinPerfCalibrate(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	// -1 for named
	ctimeS := calibrateCTimeSigma(ts1, linuxsched.GetNCores()-1, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	db.DPrintf(db.TEST, "Calibrate Linux baseline")
	// -1 for named
	ctimeL := calibrateCTimeLinux(ts1, linuxsched.GetNCores()-1, N_ITER)
	db.DPrintf(db.TEST, "Linux baseline compute time: %v", ctimeL)

	err := ts1.Remove()
	assert.Nil(t, err)

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
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	// - 2 to account for NAMED reserved cores
	ctimeS := calibrateCTimeSigma(ts1, linuxsched.GetNCores()-2, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	c := make(chan time.Duration)
	go runSpinPerf(ts1, c, 0, linuxsched.GetNCores()-2, N_ITER, "spin1")
	go runSpinPerf(ts1, c, 0, linuxsched.GetNCores()-2, N_ITER, "spin2")

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

	err := ts1.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestSpinPerfDoubleBEandLC(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	// - 2 to account for NAMED reserved cores
	ctimeS := calibrateCTimeSigma(ts1, linuxsched.GetNCores()-2, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	beC := make(chan time.Duration)
	lcC := make(chan time.Duration)
	// - 2 to account for NAMED reserved cores
	go runSpinPerf(ts1, lcC, proc.Tmcpu(1000*(linuxsched.GetNCores()-2)), linuxsched.GetNCores()-2, N_ITER, "lcspin")
	go runSpinPerf(ts1, beC, 0, linuxsched.GetNCores()-2, N_ITER, "bespin")

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

	err := ts1.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestSpinPerfDoubleBEandLCMultiRealm(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	ts1 := test.NewRealmTstate(rootts, REALM1)
	ts2 := test.NewRealmTstate(rootts, REALM2)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	// - 2 to account for NAMED reserved cores
	ctimeS := calibrateCTimeSigma(ts1, linuxsched.GetNCores()-2, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	beC := make(chan time.Duration)
	lcC := make(chan time.Duration)
	// - 2 to account for NAMED reserved cores
	go runSpinPerf(ts1, lcC, proc.Tmcpu(1000*(linuxsched.GetNCores()-2)), linuxsched.GetNCores()-2, N_ITER, "lcspin")
	go runSpinPerf(ts2, beC, 0, linuxsched.GetNCores()-2, N_ITER, "bespin")

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

	err := ts1.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}
