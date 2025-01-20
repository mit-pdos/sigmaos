package clnt_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	proto "sigmaos/apps/cache/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/linux/mem"
	rd "sigmaos/util/rand"
)

const (
	SLEEP_MSECS = 2000
	N_ITER      = 5_000_000_000
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
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	db.DPrintf(db.TEST, "Local ip: %v", mrts.GetRealm(test.REALM1).ProcEnv().GetInnerContainerIP())

	sts1, err := mrts.GetRoot().GetDir(sp.MSCHED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	sts, err := mrts.GetRealm(test.REALM1).GetDir(sp.NAMED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm named root %v\n", sp.Names(sts))

	assert.True(t, sp.Present(sts, []string{sp.UXREL}), "initfs")

	sts, err = mrts.GetRealm(test.REALM1).GetDir(sp.MSCHED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "realm names sched %v\n", sp.Names(sts))

	assert.True(t, sts1[0].Name == sts[0].Name)
}

func TestBasicMultiRealmSingleNode(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1, test.REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	mscheds1, err := mrts.GetRealm(test.REALM1).GetDir(sp.MSCHED)
	assert.Nil(t, err)
	// Only one msched so far.
	assert.True(mrts.T, len(mscheds1) == 1, "Wrong number mscheds %v", mscheds1)

	mscheds2, err := mrts.GetRealm(test.REALM2).GetDir(sp.MSCHED)
	assert.Nil(t, err)
	// Only one msched so far.
	assert.True(mrts.T, len(mscheds2) == 1, "Wrong number mscheds %v", mscheds2)

	for i := range mscheds1 {
		assert.Equal(t, mscheds1[i].Name, mscheds2[i].Name)
	}
}

func TestBasicMultiRealmMultiNode(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1, test.REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	mrts.GetRoot().BootNode(1)

	time.Sleep(2 * sp.Conf.Realm.KERNEL_SRV_REFRESH_INTERVAL)

	m1, err3 := mrts.GetRealm(test.REALM1).GetNamedEndpoint()
	assert.Nil(t, err3, "GetNamedEndpoint: %v", err3)
	db.DPrintf(db.TEST, "[%v] named addr: %v", test.REALM1, m1)
	m2, err3 := mrts.GetRealm(test.REALM2).GetNamedEndpoint()
	assert.Nil(t, err3, "GetNamedEndpoint: %v", err3)
	db.DPrintf(db.TEST, "[%v] named addr: %v", test.REALM2, m2)

	mscheds1, err := mrts.GetRealm(test.REALM1).GetDir(sp.MSCHED)
	assert.Nil(t, err, "ErrGetDir MSCHED: %v", err)
	assert.True(mrts.T, len(mscheds1) == 2, "Wrong number mscheds %v", mscheds1)

	mscheds2, err := mrts.GetRealm(test.REALM2).GetDir(sp.MSCHED)
	assert.Nil(t, err, "ErrGetDir MSCHED: %v", err)
	assert.True(mrts.T, len(mscheds2) == 2, "Wrong number mscheds %v", mscheds2)

	for i := range mscheds1 {
		assert.Equal(t, mscheds1[i].Name, mscheds2[i].Name)
	}
}

func TestWaitExitSimpleSingle(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	sts1, err := mrts.GetRoot().GetDir(sp.MSCHED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", mrts.GetRealm(test.REALM1).ProcEnv().GetInnerContainerIP())

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong: %v", status)

	for _, d := range []string{sp.S3, sp.UX} {
		sts1, err := mrts.GetRoot().GetDir(d)
		assert.Nil(t, err)
		assert.True(t, len(sts1) == 1, "No %vs in root realm", d)
		sts, err := mrts.GetRealm(test.REALM1).GetDir(d)
		db.DPrintf(db.TEST, "realm names %v %v\n", d, sp.Names(sts))
		assert.Nil(t, err)
		assert.True(t, len(sts) == 1, "No %vs in user realm", d)
		for _, st := range sts1 {
			// If there is a name in common in the directory, check that they are for different endpoints
			if sp.Present(sts, []string{st.Name}) {
				ep, err := mrts.GetRealm(test.REALM1).ReadEndpoint(filepath.Join(d, st.Name))
				assert.Nil(t, err, "ReadEndpoint: %v", err)
				ep1, err := mrts.GetRoot().ReadEndpoint(filepath.Join(d, st.Name))
				assert.Nil(t, err, "ReadEndpoint: %v", err)
				assert.False(t, ep.Addrs()[0] == ep1.Addrs()[0], "%v cross-over", d)
			}
		}
	}
}

func TestBasicFairness(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	time.Sleep(2 * sp.Conf.Realm.KERNEL_SRV_REFRESH_INTERVAL)

	p1 := proc.NewProc("sleeper", []string{"100000s", "name/"})
	p1.SetMem(mem.GetTotalMem()/2 + 1)

	db.DPrintf(db.TEST, "Spawn big realm's proc")
	err := mrts.GetRealm(test.REALM1).Spawn(p1)
	assert.Nil(mrts.T, err, "Err spawn: %v", err)
	err = mrts.GetRealm(test.REALM1).WaitStart(p1.GetPid())
	assert.Nil(mrts.T, err, "Err WaitStart: %v", err)
	db.DPrintf(db.TEST, "Big realm's proc started")

	err = mrts.AddRealm(test.REALM2)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Created realm 2")

	p2 := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	p2.SetMem(mem.GetTotalMem())

	db.DPrintf(db.TEST, "Spawn small realm's proc")
	err = mrts.GetRealm(test.REALM2).Spawn(p2)
	assert.Nil(mrts.T, err, "Err spawn: %v", err)
	err = mrts.GetRealm(test.REALM2).WaitStart(p2.GetPid())
	assert.Nil(mrts.T, err, "Err WaitStart: %v", err)
	db.DPrintf(db.TEST, "Small realm's proc started")

	status, err := mrts.GetRealm(test.REALM1).WaitExit(p1.GetPid())
	assert.Nil(mrts.T, err, "Err WaitExit: %v", err)
	assert.True(mrts.T, status.IsStatusEvicted(), "Wrong status: %v", status)

	status, err = mrts.GetRealm(test.REALM2).WaitExit(p2.GetPid())
	assert.Nil(mrts.T, err, "Err WaitExit: %v", err)
	assert.True(mrts.T, status.IsStatusOK(), "Wrong status: %v", status)
}

// May not work if running with non-local build, because we only start one S3
// server but the proc may be spawned onto either node (and ~local resolution
// will return ErrNotFound when fetching from the S3 Origin if using remote
// builds on the node without an S3 server).
func TestWaitExitMultiNode(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	mrts.GetRoot().BootNode(1)
	subsysCnts := []int64{1, 2}
	err1 = mrts.AddRealmNumSubsystems(test.REALM1, subsysCnts[0], subsysCnts[1])
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts1, err := mrts.GetRoot().GetDir(sp.MSCHED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", mrts.GetRealm(test.REALM1).ProcEnv().GetInnerContainerIP())

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusOK(), "Exit status wrong: %v", status)

	for i, d := range []string{sp.S3, sp.UX} {
		sts1, err := mrts.GetRoot().GetDir(d)
		assert.Nil(t, err)
		assert.True(t, len(sts1) == 2, "No %vs in root realm", d)
		sts, err := mrts.GetRealm(test.REALM1).GetDir(d)
		db.DPrintf(db.TEST, "realm names %v %v\n", d, sp.Names(sts))
		assert.Nil(t, err)
		assert.True(t, int64(len(sts)) == subsysCnts[i], "Wrong number of %vs in user realm: %v != %v", d, len(sts), subsysCnts[i])
		for _, st := range sts1 {
			// If there is a name in common in the directory, check that they are for different endpoints
			if sp.Present(sts, []string{st.Name}) {
				ep, err := mrts.GetRealm(test.REALM1).ReadEndpoint(filepath.Join(d, st.Name))
				assert.Nil(t, err, "ReadEndpoint: %v", err)
				ep1, err := mrts.GetRoot().ReadEndpoint(filepath.Join(d, st.Name))
				assert.Nil(t, err, "ReadEndpoint: %v", err)
				assert.False(t, ep.Addrs()[0] == ep1.Addrs()[0], "%v cross-over", d)
			}
		}
	}
}

func TestEvictSingle(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	sts1, err := mrts.GetRoot().GetDir(sp.MSCHED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", mrts.GetRealm(test.REALM1).ProcEnv().GetInnerContainerIP())

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", 60000), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitstart")
	err = mrts.GetRealm(test.REALM1).WaitStart(a.GetPid())
	db.DPrintf(db.TEST, "Post waitstart")
	assert.Nil(t, err, "waitstart error")

	db.DPrintf(db.TEST, "Pre evict")
	err = mrts.GetRealm(test.REALM1).Evict(a.GetPid())
	db.DPrintf(db.TEST, "Post evict")
	assert.Nil(t, err, "evict error")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)
}

func TestEvictMultiRealm(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1, test.REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	sts1, err := mrts.GetRoot().GetDir(sp.MSCHED)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "names sched %v\n", sp.Names(sts1))

	db.DPrintf(db.TEST, "Local ip: %v", mrts.GetRealm(test.REALM1).ProcEnv().GetInnerContainerIP())

	a := proc.NewProc("sleeper", []string{fmt.Sprintf("%dms", 60000), "name/"})
	db.DPrintf(db.TEST, "Pre spawn")
	err = mrts.GetRealm(test.REALM1).Spawn(a)
	assert.Nil(t, err, "Error spawn: %v", err)
	db.DPrintf(db.TEST, "Post spawn")

	db.DPrintf(db.TEST, "Pre waitstart")
	err = mrts.GetRealm(test.REALM1).WaitStart(a.GetPid())
	db.DPrintf(db.TEST, "Post waitstart")
	assert.Nil(t, err, "waitstart error")

	db.DPrintf(db.TEST, "Pre evict")
	err = mrts.GetRealm(test.REALM1).Evict(a.GetPid())
	db.DPrintf(db.TEST, "Post evict")
	assert.Nil(t, err, "evict error")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(a.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(t, err, "WaitExit error")
	assert.True(t, status.IsStatusEvicted(), "Exit status wrong: %v", status)
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

// Test that realms can't access kernel services they shouldn't be able to
// access.
func TestKernelIsolationBasic(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1, test.REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	// Get the root named endpoint
	rootNamedEP, err := mrts.GetRoot().GetNamedEndpoint()
	assert.Nil(t, err, "Err %v", err)
	db.DPrintf(db.TEST, "rootNamed EP: %v", rootNamedEP)
	pn := filepath.Join(sp.NAME, sp.BESCHEDREL) + "/"
	db.DPrintf(db.TEST, "Try to get dir %v", pn)
	// Ensure that tenant realms can perform GetDir on union directories which
	// live in the root named (and are mounted into the tenant's named)
	sts, err := mrts.GetRealm(test.REALM1).GetDir(pn)
	assert.Nil(t, err, "Unable to GetDir root-mounted union dir %v: %v", pn, err)
	assert.True(t, len(sts) == 1, "Wrong list of mscheds: %v", sp.Names(sts))
	sts1, err := mrts.GetRealm(test.REALM1).GetDir(filepath.Join(pn, sts[0].Name) + "/")
	assert.Nil(t, err, "Unable to GetDir root-mounted union dir %v: %v", pn, err)
	assert.True(t, len(sts1) == 2, "Wrong procq contents: %v", sp.Names(sts1))
	db.DPrintf(db.TEST, "Got contents of %v%v: %v", pn, sts[0].Name, sp.Names(sts1))
	// Ensure that tenant realms can't access the root named's root directory
	err = mrts.GetRealm(test.REALM1).MountTree(rootNamedEP, "", "name/rootnamed")
	assert.NotNil(t, err, "Able to mount name/ from root named")
	// Ensure that tenant realms can't access union dirs which aren't mounted into their realms
	err = mrts.GetRealm(test.REALM1).MountTree(rootNamedEP, "s3", "name/roots3")
	assert.NotNil(t, err, "Able to mount name/s3 from root named")

	// Get the ID of the kernel clnt
	kid := mrts.GetRoot().GetKernelClnt(0).KernelId()
	// Read the kernelsrv endpoint
	ksrvEP, err := mrts.GetRoot().ReadEndpoint(filepath.Join(sp.BOOT, kid))
	assert.Nil(t, err, "Err %v", err)
	db.DPrintf(db.TEST, "KernelSrv EP: %v", ksrvEP)

	// No realm should be able to access kernelsrvs
	err = mrts.GetRealm(test.REALM1).MountTree(ksrvEP, "", "name/kernelsrv")
	db.DPrintf(db.TEST, "MountTree kernelsrv err %v", err)
	assert.NotNil(t, err, "Able to mount kernelsrv")
	err = mrts.GetRealm(test.REALM2).MountTree(ksrvEP, "", "name/kernelsrv")
	db.DPrintf(db.TEST, "MountTree kernelsrv err %v", err)
	assert.NotNil(t, err, "Able to mount kernelsrv")

	// Read an s3 endpoint from realm 1
	s3r1EP, err := mrts.GetRealm(test.REALM1).ReadEndpoint(filepath.Join(sp.S3, sp.ANY))
	assert.Nil(t, err, "Err %v", err)
	db.DPrintf(db.TEST, "S3 Realm1 EP: %v", s3r1EP)
	// Make sure ts1 can mount its realm's S3 server
	err = mrts.GetRealm(test.REALM1).MountTree(s3r1EP, "", "name/s3r1")
	assert.Nil(t, err, "Unable to mount s3r1")
	// Make sure ts2 can't connect to realm1's S3 server
	err = mrts.GetRealm(test.REALM2).MountTree(s3r1EP, "", "name/s3r1")
	assert.NotNil(t, err, "Able to mount s3r1")

	// Read a ux endpoint from realm 2
	uxr2EP, err := mrts.GetRealm(test.REALM2).ReadEndpoint(filepath.Join(sp.UX, sp.ANY))
	assert.Nil(t, err, "Err %v", err)
	db.DPrintf(db.TEST, "UX Realm2 EP: %v", uxr2EP)
	// Make sure ts2 can mount its realm's UX server
	err = mrts.GetRealm(test.REALM2).MountTree(uxr2EP, "", "name/uxr2")
	assert.Nil(t, err, "Unable to mount uxr2")
	// Make sure ts1 can't connect to realm2's UX server
	err = mrts.GetRealm(test.REALM1).MountTree(uxr2EP, "", "name/uxr2")
	assert.NotNil(t, err, "Able to mount uxr2")
}

// Test basic realm isolation: start a cached in realm1 and check that
// it isn't visible in realm2.
func TestMultiRealmIsolationBasic(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1, test.REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	job := rd.String(16)
	cm, err := cachegrpmgr.NewCacheMgr(mrts.GetRealm(test.REALM1).SigmaClnt, job, 1, 0, true)
	assert.Nil(t, err)

	cc1 := cachegrpclnt.NewCachedSvcClnt(mrts.GetRealm(test.REALM1).FsLib, job)

	err = cc1.Put("hello", &proto.CacheString{Val: "hello"})
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "newcacheclnt %v", mrts.GetRealm(test.REALM2).FsLib)

	cc2 := cachegrpclnt.NewCachedSvcClnt(mrts.GetRealm(test.REALM2).FsLib, job)

	db.DPrintf(db.TEST, "About to stat srvs")

	// Check that there is no cached in ts2
	_, err = cc2.StatsSrvs()
	assert.NotNil(t, err)

	db.DPrintf(db.TEST, "Done stat srvs")

	cm.Stop()

	db.DPrintf(db.TEST, "Done cached stop")
}

// Take endpoint from realm1 and make an enpoint file for it in realm2
// and access the endpoint from realm2.
func TestMultiRealmIsolationEndpoint(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1, test.REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	job := rd.String(16)
	cm, err := cachegrpmgr.NewCacheMgr(mrts.GetRealm(test.REALM1).SigmaClnt, job, 1, 0, true)
	assert.Nil(t, err)

	cc1 := cachegrpclnt.NewCachedSvcClnt(mrts.GetRealm(test.REALM1).FsLib, job)

	err = cc1.Put("hello", &proto.CacheString{Val: "hello"})
	assert.Nil(t, err)

	ep, err := mrts.GetRealm(test.REALM1).ReadEndpoint(cc1.Server(0))
	assert.Nil(t, err, "Err %v", err)

	db.DPrintf(db.TEST, "ep %v", ep)

	// Remove public port
	if len(ep.Addrs()) > 1 {
		ep.SetAddr(ep.Addrs()[:1])
	}

	pn := filepath.Join(sp.NAMED, "srv")
	err = mrts.GetRealm(test.REALM2).MkEndpointFile(pn, ep)
	assert.Nil(t, err)
	pn = pn + "/"

	status := spawnDirreader(mrts.GetRealm(test.REALM2), pn)
	assert.True(t, status.IsStatusErr(), "Status is: %v", status)
	db.DPrintf(db.TEST, "status %v %v\n", status.Msg(), status.Data())

	cm.Stop()
}

func TestMultiRealmIsolationNamed(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1, test.REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	eproot, err := mrts.GetRoot().GetNamedEndpoint()
	assert.Nil(t, err, "Err %v", err)

	db.DPrintf(db.TEST, "rootep %v", eproot)

	ep1, err := mrts.GetRealm(test.REALM1).GetNamedEndpoint()
	assert.Nil(t, err, "Err %v", err)

	db.DPrintf(db.TEST, "rootep %v", ep1)

	pn := filepath.Join(sp.NAMED, "rootnamed")
	err = mrts.GetRealm(test.REALM2).MkEndpointFile(pn, ep1)
	assert.Nil(t, err)

	pn = filepath.Join(sp.NAMED, "named1")
	err = mrts.GetRealm(test.REALM1).MkEndpointFile(pn, ep1)
	assert.Nil(t, err)

	sts, err := mrts.GetRealm(test.REALM2).GetDir(sp.NAMED)
	assert.Nil(t, err, "GetDir %v %v\n", sp.NAMED, sp.Names(sts))

	db.DPrintf(db.TEST, "ts2 %v", sp.Names(sts))

	err = mrts.GetRealm(test.REALM2).MkEndpointFile(pn, ep1)
	assert.Nil(t, err)

	pn = filepath.Join(sp.NAMED, "named1"+"/")
	status := spawnDirreader(mrts.GetRealm(test.REALM1), pn)
	assert.True(t, status.IsStatusOK(), "%v: Status is: %v", pn, status)
	db.DPrintf(db.TEST, "status %v %v\n", status.Msg(), status.Data())

	for _, f := range []string{"rootnamed", "named1"} {
		pn := filepath.Join(sp.NAMED, f) + "/"
		status := spawnDirreader(mrts.GetRealm(test.REALM2), pn)
		assert.True(t, status.IsStatusErr(), "%v: Status is: %v", pn, status)
		db.DPrintf(db.TEST, "status %v %v\n", status.Msg(), status.Data())
		sts, err := mrts.GetRealm(test.REALM2).GetDir(pn)
		assert.NotNil(t, err, "GetDir %v %v\n", pn, sp.Names(sts))
	}
}

// These often don't pass due to a scheduling bug in Linux.
//func TestSpinPerfCalibrate(t *testing.T) {
//	rootts, err1 := test.NewTstateWithRealms(t)
//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
//		return
//	}
//	ts1, err1 := test.NewRealmTstate(rootts, test.REALM1)
//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
//		return
//	}
//
//	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
//	// -1 for named
//	ctimeS := calibrateCTimeSigma(ts1, linuxsched.GetNCores()-1, N_ITER)
//	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)
//
//	db.DPrintf(db.TEST, "Calibrate Linux baseline")
//	// -1 for named
//	ctimeL := calibrateCTimeLinux(ts1, linuxsched.GetNCores()-1, N_ITER)
//	db.DPrintf(db.TEST, "Linux baseline compute time: %v", ctimeL)
//
//	err := ts1.Remove()
//	assert.Nil(t, err)
//
//	rootts.Shutdown()
//}
//
//// Calculate slowdown %
//func slowdown(baseline, dur time.Duration) float64 {
//	return float64(dur) / float64(baseline)
//}
//
//func targetTime(baseline time.Duration, tslowdown float64) time.Duration {
//	return time.Duration(float64(baseline) * tslowdown)
//}
//
//// May fail on Linux systems (especially when they have multiple NUMA
//// nodes), due to a linux scheduler bug. See:
//// https://www.usenix.org/system/files/login/articles/login_winter16_02_lozi.pdf
//func TestSpinPerfDoubleSlowdown(t *testing.T) {
//	rootts, err1 := test.NewTstateWithRealms(t)
//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
//		return
//	}
//	ts1, err1 := test.NewRealmTstate(rootts, test.REALM1)
//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
//		return
//	}
//
//	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
//	// - 2 to account for NAMED reserved cores
//	ctimeS := calibrateCTimeSigma(ts1, linuxsched.GetNCores()-2, N_ITER)
//	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)
//
//	c := make(chan time.Duration)
//	go runSpinPerf(ts1, c, 0, linuxsched.GetNCores()-2, N_ITER, "spin1")
//	go runSpinPerf(ts1, c, 0, linuxsched.GetNCores()-2, N_ITER, "spin2")
//
//	d1 := <-c
//	d2 := <-c
//
//	// Calculate slowdown
//	d1sd := slowdown(ctimeS, d1)
//	d2sd := slowdown(ctimeS, d2)
//
//	// Target slowdown (x)
//	tsd := 1.70
//
//	// Check that execution time matches target time.
//	assert.True(mrts.T, d1sd > tsd, "Spin perf 1 not enough slowdown (%v): %v <= %v", d1sd, d1, targetTime(ctimeS, tsd))
//	assert.True(mrts.T, d2sd > tsd, "Spin perf 2 not enough slowdown (%v): %v <= %v", d1sd, d2, targetTime(ctimeS, tsd))
//
//	err := ts1.Remove()
//	assert.Nil(t, err)
//
//	rootts.Shutdown()
//}
//
//func TestSpinPerfDoubleBEandLC(t *testing.T) {
//	// Bail out early if machine has too many cores (which messes with the cgroups setting)
//	if !assert.False(t, linuxsched.GetNCores() > 10, "SpawnBurst test will fail because machine has >10 cores, which causes cgroups settings to fail") {
//		return
//	}
//	rootts, err1 := test.NewTstateWithRealms(t)
//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
//		return
//	}
//	ts1, err1 := test.NewRealmTstate(rootts, test.REALM1)
//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
//		return
//	}
//
//	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
//	// - 2 to account for NAMED reserved cores
//	ctimeS := calibrateCTimeSigma(ts1, linuxsched.GetNCores()-2, N_ITER)
//	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)
//
//	beC := make(chan time.Duration)
//	lcC := make(chan time.Duration)
//	// - 2 to account for NAMED reserved cores
//	go runSpinPerf(ts1, lcC, proc.Tmcpu(1000*(linuxsched.GetNCores()-2)), linuxsched.GetNCores()-2, N_ITER, "lcspin")
//	go runSpinPerf(ts1, beC, 0, linuxsched.GetNCores()-2, N_ITER, "bespin")
//
//	durBE := <-beC
//	durLC := <-lcC
//
//	// Calculate slodown
//	beSD := slowdown(ctimeS, durBE)
//	lcSD := slowdown(ctimeS, durLC)
//
//	// Target slowdown (x)
//	beMinSD := 1.5
//	beMaxSD := 2.5
//	lcMaxSD := 1.1
//
//	// Check that execution time matches target time.
//	assert.True(mrts.T, lcSD <= lcMaxSD, "LC too much slowdown (%v): %v > %v", lcSD, durLC, targetTime(ctimeS, lcMaxSD))
//	assert.True(mrts.T, beSD <= beMaxSD, "BE too much slowdown (%v): %v > %v", beSD, durBE, targetTime(ctimeS, beMaxSD))
//	assert.True(mrts.T, beSD > beMinSD, "BE not enough slowdown (%v): %v < %v", beSD, durBE, targetTime(ctimeS, beMinSD))
//
//	err := ts1.Remove()
//	assert.Nil(t, err)
//
//	rootts.Shutdown()
//}
//
//func TestSpinPerfDoubleBEandLCMultiRealm(t *testing.T) {
//	// Bail out early if machine has too many cores (which messes with the cgroups setting)
//	if !assert.False(t, linuxsched.GetNCores() > 10, "SpawnBurst test will fail because machine has >10 cores, which causes cgroups settings to fail") {
//		return
//	}
//	ts := newMultiRealmTstate(t)
//
//	db.DPrintf(db.TEST, "Calibrate SigmaOS baseline")
//	// - 2 to account for NAMED reserved cores
//	ctimeS := calibrateCTimeSigma(ts.ts1, linuxsched.GetNCores()-2, N_ITER)
//	db.DPrintf(db.TEST, "SigmaOS baseline compute time: %v", ctimeS)
//
//	beC := make(chan time.Duration)
//	lcC := make(chan time.Duration)
//	// - 2 to account for NAMED reserved cores
//	go runSpinPerf(ts.ts1, lcC, proc.Tmcpu(1000*(linuxsched.GetNCores()-2)), linuxsched.GetNCores()-2, N_ITER, "lcspin")
//	go runSpinPerf(ts.ts2, beC, 0, linuxsched.GetNCores()-2, N_ITER, "bespin")
//
//	durBE := <-beC
//	durLC := <-lcC
//
//	// Calculate slodown
//	beSD := slowdown(ctimeS, durBE)
//	lcSD := slowdown(ctimeS, durLC)
//
//	// Target slowdown (x)
//	beMinSD := 1.5
//	beMaxSD := 2.5
//	lcMaxSD := 1.1
//
//	// Check that execution time matches target time.
//	assert.True(t, lcSD <= lcMaxSD, "LC too much slowdown (%v): %v > %v", lcSD, durLC, targetTime(ctimeS, lcMaxSD))
//	assert.True(t, beSD <= beMaxSD, "BE too much slowdown (%v): %v > %v", beSD, durBE, targetTime(ctimeS, beMaxSD))
//	assert.True(t, beSD > beMinSD, "BE not enough slowdown (%v): %v < %v", beSD, durBE, targetTime(ctimeS, beMinSD))
//
//}
