package realmclnt_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func TestCompile(t *testing.T) {
}

func TestBasicSimple(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	db.DPrintf(db.TEST, "Local ip: %v", ts1.ProcEnv().GetInnerContainerIP())

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	sts, err := ts1.GetDir(sp.NAMED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm named root %v\n", sp.Names(sts))

	assert.True(t, sp.Present(sts, []string{sp.UXREL}), "initfs")

	sts, err = ts1.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm names sched %v\n", sp.Names(sts))

	assert.True(t, sts1[0].Name == sts[0].Name)

	err = ts1.Remove()
	assert.Nil(t, err, "Error Remove: %v", err)

	rootts.Shutdown()
}

type realmTstate struct {
	rootts *test.Tstate
	ts1    *test.RealmTstate
	ts2    *test.RealmTstate
}

func newMultiRealmTstate(t *testing.T) *realmTstate {
	ts := &realmTstate{}
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return nil
	}
	ts.rootts = rootts
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return nil
	}
	ts.ts1 = ts1
	ts2, err1 := test.NewRealmTstate(rootts, REALM2)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return nil
	}
	ts.ts2 = ts2
	db.DPrintf(db.TEST, "[%v] Local ip: %v", REALM1, ts1.ProcEnv().GetInnerContainerIP())
	db.DPrintf(db.TEST, "[%v] Local ip: %v", REALM2, ts2.ProcEnv().GetInnerContainerIP())
	return ts
}

func (ts *realmTstate) shutdown() {
	err := ts.ts1.Remove()
	assert.Nil(ts.rootts.T, err)
	err = ts.ts2.Remove()
	assert.Nil(ts.rootts.T, err)
	ts.rootts.Shutdown()
}

func TestBasicMultiRealmSingleNode(t *testing.T) {
	ts := newMultiRealmTstate(t)
	schedds1, err := ts.ts1.GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	// Only one schedd so far.
	assert.True(ts.rootts.T, len(schedds1) == 1, "Wrong number schedds %v", schedds1)

	schedds2, err := ts.ts2.GetDir(sp.SCHEDD)
	assert.Nil(t, err)
	// Only one schedd so far.
	assert.True(ts.rootts.T, len(schedds2) == 1, "Wrong number schedds %v", schedds2)

	for i := range schedds1 {
		assert.Equal(t, schedds1[i].Name, schedds2[i].Name)
	}
	ts.shutdown()
}

func TestBasicMultiRealmMultiNode(t *testing.T) {
	ts := newMultiRealmTstate(t)
	ts.rootts.BootNode(1)

	time.Sleep(2 * sp.Conf.Realm.KERNEL_SRV_REFRESH_INTERVAL)

	m1, err3 := ts.ts1.GetNamedEndpoint()
	assert.Nil(t, err3, "GetNamedEndpoint: %v", err3)
	db.DPrintf(db.TEST, "[%v] named addr: %v", REALM1, m1)
	m2, err3 := ts.ts2.GetNamedEndpoint()
	assert.Nil(t, err3, "GetNamedEndpoint: %v", err3)
	db.DPrintf(db.TEST, "[%v] named addr: %v", REALM2, m2)

	schedds1, err := ts.ts1.GetDir(sp.SCHEDD)
	assert.Nil(t, err, "ErrGetDir SCHEDD: %v", err)
	assert.True(ts.rootts.T, len(schedds1) == 2, "Wrong number schedds %v", schedds1)

	schedds2, err := ts.ts2.GetDir(sp.SCHEDD)
	assert.Nil(t, err, "ErrGetDir SCHEDD: %v", err)
	assert.True(ts.rootts.T, len(schedds2) == 2, "Wrong number schedds %v", schedds2)

	for i := range schedds1 {
		assert.Equal(t, schedds1[i].Name, schedds2[i].Name)
	}

	ts.shutdown()
}

func TestBasicMultiRealmFairness(t *testing.T) {
	ts := newMultiRealmTstate(t)

	time.Sleep(2 * sp.Conf.Realm.KERNEL_SRV_REFRESH_INTERVAL)

	p1 := proc.NewProc("sleeper", []string{"100000s", "name/"})
	p1.SetMem(mem.GetTotalMem()/2 + 1)

	db.DPrintf(db.TEST, "Spawn big realm's proc")
	err := ts.ts1.Spawn(p1)
	assert.Nil(ts.rootts.T, err, "Err spawn: %v", err)
	err = ts.ts1.WaitStart(p1.GetPid())
	assert.Nil(ts.rootts.T, err, "Err WaitStart: %v", err)
	db.DPrintf(db.TEST, "Big realm's proc started")

	time.Sleep(2 * sp.Conf.Realm.KERNEL_SRV_REFRESH_INTERVAL)

	p2 := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	p2.SetMem(mem.GetTotalMem())

	db.DPrintf(db.TEST, "Spawn small realm's proc")
	err = ts.ts2.Spawn(p2)
	assert.Nil(ts.rootts.T, err, "Err spawn: %v", err)
	err = ts.ts2.WaitStart(p2.GetPid())
	assert.Nil(ts.rootts.T, err, "Err WaitStart: %v", err)
	db.DPrintf(db.TEST, "Small realm's proc started")

	status, err := ts.ts1.WaitExit(p1.GetPid())
	assert.Nil(ts.rootts.T, err, "Err WaitExit: %v", err)
	assert.True(ts.rootts.T, status.IsStatusEvicted(), "Wrong status: %v", status)

	status, err = ts.ts2.WaitExit(p2.GetPid())
	assert.Nil(ts.rootts.T, err, "Err WaitExit: %v", err)
	assert.True(ts.rootts.T, status.IsStatusOK(), "Wrong status: %v", status)

	ts.shutdown()
}

func TestWaitExitSimpleSingle(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts1.ProcEnv().GetInnerContainerIP())

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

	for _, d := range []string{sp.S3, sp.UX} {
		sts1, err := rootts.GetDir(d)
		assert.Nil(t, err)
		assert.True(t, len(sts1) == 1, "No %vs in root realm", d)
		sts, err := ts1.GetDir(d)
		db.DPrintf(db.TEST, "realm names %v %v\n", d, sp.Names(sts))
		assert.Nil(t, err)
		assert.True(t, len(sts) == 1, "No %vs in user realm", d)
		for _, st := range sts1 {
			// If there is a name in common in the directory, check that they are for different endpoints
			if sp.Present(sts, []string{st.Name}) {
				ep, err := ts1.ReadEndpoint(filepath.Join(d, st.Name))
				assert.Nil(t, err, "ReadEndpoint: %v", err)
				ep1, err := rootts.ReadEndpoint(filepath.Join(d, st.Name))
				assert.Nil(t, err, "ReadEndpoint: %v", err)
				assert.False(t, ep.Addrs()[0] == ep1.Addrs()[0], "%v cross-over", d)
			}
		}
	}

	err = ts1.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestWaitExitMultiNode(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rootts.BootNode(1)
	subsysCnts := []int64{2, 1}
	ts1, err1 := test.NewRealmTstateNumSubsystems(rootts, REALM1, subsysCnts[0], subsysCnts[1])
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts1.ProcEnv().GetInnerContainerIP())

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

	for i, d := range []string{sp.S3, sp.UX} {
		sts1, err := rootts.GetDir(d)
		assert.Nil(t, err)
		assert.True(t, len(sts1) == 2, "No %vs in root realm", d)
		sts, err := ts1.GetDir(d)
		db.DPrintf(db.TEST, "realm names %v %v\n", d, sp.Names(sts))
		assert.Nil(t, err)
		assert.True(t, int64(len(sts)) == subsysCnts[i], "Wrong number of %vs in user realm: %v != %v", d, len(sts), subsysCnts[i])
		for _, st := range sts1 {
			// If there is a name in common in the directory, check that they are for different endpoints
			if sp.Present(sts, []string{st.Name}) {
				ep, err := ts1.ReadEndpoint(filepath.Join(d, st.Name))
				assert.Nil(t, err, "ReadEndpoint: %v", err)
				ep1, err := rootts.ReadEndpoint(filepath.Join(d, st.Name))
				assert.Nil(t, err, "ReadEndpoint: %v", err)
				assert.False(t, ep.Addrs()[0] == ep1.Addrs()[0], "%v cross-over", d)
			}
		}
	}

	err = ts1.Remove()
	assert.Nil(t, err)

	rootts.Shutdown()
}

func TestEvictSingle(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts1, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts1.ProcEnv().GetInnerContainerIP())

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
	ts := newMultiRealmTstate(t)
	sts1, err := ts.rootts.GetDir(sp.SCHEDD)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", ts.ts1.ProcEnv().GetInnerContainerIP())

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", 60000), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = ts.ts1.Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitstart")
	err = ts.ts1.WaitStart(a.GetPid())
	db.DPrintf(db.TEST, "Post waitstart")
	assert.Nil(t, err, "waitstart error")

	db.DPrintf(db.TEST, "Pre evict")
	err = ts.ts1.Evict(a.GetPid())
	db.DPrintf(db.TEST, "Post evict")
	assert.Nil(t, err, "evict error")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ts.ts1.WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)

	ts.shutdown()
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

// Test basic realm isolation: start a cached in realm1 and check that
// it isn't visible in realm2.
func TestMultiRealmIsolationBasic(t *testing.T) {
	ts := newMultiRealmTstate(t)
	job := rd.String(16)
	cm, err := cachedsvc.NewCacheMgr(ts.ts1.SigmaClnt, job, 1, 0, true)
	assert.Nil(t, err)

	cc1 := cachedsvcclnt.NewCachedSvcClnt([]*fslib.FsLib{ts.ts1.FsLib}, job)

	err = cc1.Put("hello", &proto.CacheString{Val: "hello"})
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "newcacheclnt %v", ts.ts2.FsLib)

	cc2 := cachedsvcclnt.NewCachedSvcClnt([]*fslib.FsLib{ts.ts2.FsLib}, job)

	// Check that there is no cached in ts2
	_, err = cc2.StatsSrvs()
	assert.NotNil(t, err)

	cm.Stop()

	ts.shutdown()
}

// Take endpoint from realm1 and make an enpoint file for it in realm2
// and access the endpoint from realm2.
func TestMultiRealmIsolationEndpoint(t *testing.T) {
	ts := newMultiRealmTstate(t)
	job := rd.String(16)
	cm, err := cachedsvc.NewCacheMgr(ts.ts1.SigmaClnt, job, 1, 0, true)
	assert.Nil(t, err)

	cc1 := cachedsvcclnt.NewCachedSvcClnt([]*fslib.FsLib{ts.ts1.FsLib}, job)

	err = cc1.Put("hello", &proto.CacheString{Val: "hello"})
	assert.Nil(t, err)

	ep, err := ts.ts1.ReadEndpoint(cc1.Server(0))
	assert.Nil(t, err, "Err %v", err)

	db.DPrintf(db.TEST, "ep %v", ep)

	// Remove public port
	if len(ep.Addrs()) > 1 {
		ep.SetAddr(ep.Addrs()[:1])
	}

	pn := filepath.Join(sp.NAMED, "srv")
	err = ts.ts2.MkEndpointFile(pn, ep)
	assert.Nil(t, err)
	pn = pn + "/"

	status := spawnDirreader(ts.ts2, pn)
	assert.True(t, status.IsStatusErr(), "Status is: %v", status)
	db.DPrintf(db.TEST, "status %v %v\n", status.Msg(), status.Data())

	cm.Stop()
	ts.shutdown()
}

func TestMultiRealmIsolationNamed(t *testing.T) {
	ts := newMultiRealmTstate(t)

	eproot, err := ts.rootts.GetNamedEndpoint()
	assert.Nil(t, err, "Err %v", err)

	db.DPrintf(db.TEST, "rootep %v", eproot)

	ep1, err := ts.ts1.GetNamedEndpoint()
	assert.Nil(t, err, "Err %v", err)

	db.DPrintf(db.TEST, "rootep %v", ep1)

	pn := filepath.Join(sp.NAMED, "rootnamed")
	err = ts.ts2.MkEndpointFile(pn, ep1)
	assert.Nil(t, err)

	pn = filepath.Join(sp.NAMED, "named1")
	err = ts.ts1.MkEndpointFile(pn, ep1)
	assert.Nil(t, err)

	sts, err := ts.ts2.GetDir(sp.NAMED)
	assert.Nil(t, err, "GetDir %v %v\n", sp.NAMED, sp.Names(sts))

	db.DPrintf(db.TEST, "ts2 %v", sp.Names(sts))

	err = ts.ts2.MkEndpointFile(pn, ep1)
	assert.Nil(t, err)

	pn = filepath.Join(sp.NAMED, "named1"+"/")
	status := spawnDirreader(ts.ts1, pn)
	assert.True(t, status.IsStatusOK(), "%v: Status is: %v", pn, status)
	db.DPrintf(db.TEST, "status %v %v\n", status.Msg(), status.Data())

	for _, f := range []string{"rootnamed", "named1"} {
		pn := filepath.Join(sp.NAMED, f) + "/"
		status := spawnDirreader(ts.ts2, pn)
		assert.True(t, status.IsStatusErr(), "%v: Status is: %v", pn, status)
		db.DPrintf(db.TEST, "status %v %v\n", status.Msg(), status.Data())
		sts, err := ts.ts2.GetDir(pn)
		assert.NotNil(t, err, "GetDir %v %v\n", pn, sp.Names(sts))
	}
	ts.shutdown()
}

func TestSpinPerfCalibrate(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

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

// XXX May fail on Linux systems (especially when they have multiple NUMA
// nodes), due to a linux scheduler bug. See:
// https://www.usenix.org/system/files/login/articles/login_winter16_02_lozi.pdf
func TestSpinPerfDoubleSlowdown(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

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
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "SpawnBurst test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

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
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "SpawnBurst test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}
	ts := newMultiRealmTstate(t)

	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
	// - 2 to account for NAMED reserved cores
	ctimeS := calibrateCTimeSigma(ts.ts1, linuxsched.GetNCores()-2, N_ITER)
	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)

	beC := make(chan time.Duration)
	lcC := make(chan time.Duration)
	// - 2 to account for NAMED reserved cores
	go runSpinPerf(ts.ts1, lcC, proc.Tmcpu(1000*(linuxsched.GetNCores()-2)), linuxsched.GetNCores()-2, N_ITER, "lcspin")
	go runSpinPerf(ts.ts2, beC, 0, linuxsched.GetNCores()-2, N_ITER, "bespin")

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
	assert.True(t, lcSD <= lcMaxSD, "LC too much slowdown (%v): %v > %v", lcSD, durLC, targetTime(ctimeS, lcMaxSD))
	assert.True(t, beSD <= beMaxSD, "BE too much slowdown (%v): %v > %v", beSD, durBE, targetTime(ctimeS, beMaxSD))
	assert.True(t, beSD > beMinSD, "BE not enough slowdown (%v): %v < %v", beSD, durBE, targetTime(ctimeS, beMinSD))

	ts.shutdown()
}
