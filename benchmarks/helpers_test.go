package benchmarks_test

import (
	"io"
	"net/rpc"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/scheddclnt"
	"sigmaos/semclnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

//
// A set of helper functions that we use across our benchmarks.
//

// ========== Proc Helpers ==========

func newNProcs(n int, prog string, args []string, env map[string]string, mcpu proc.Tmcpu) ([]*proc.Proc, []interface{}) {
	ps := make([]*proc.Proc, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.NewProc(prog, args)
		for k, v := range env {
			p.AppendEnv(k, v)
		}
		p.SetMcpu(mcpu)
		ps = append(ps, p)
		is = append(is, p)
	}
	return ps, is
}

func spawnBurstProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	db.DPrintf(db.TEST, "Burst-spawning %v procs in chunks of size %v", len(ps), len(ps)/MAX_PARALLEL)
	for _, p := range ps {
		err := ts.Spawn(p)
		assert.Nil(ts.Ts.T, err, "Error Spawn: %v", err)
	}
}

func spawnProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	for _, p := range ps {
		err := ts.Spawn(p)
		assert.Nil(ts.Ts.T, err, "WaitStart: %v", err)
	}
}

func waitStartProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	for _, p := range ps {
		err := ts.WaitStart(p.GetPid())
		assert.Nil(ts.Ts.T, err, "WaitStart: %v", err)
	}
	db.DPrintf(db.TEST, "%v burst-spawned procs have all started", len(ps))
}

func waitExitProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	for _, p := range ps {
		status, err := ts.WaitExit(p.GetPid())
		if assert.Nil(ts.Ts.T, err, "WaitStart: %v", err) {
			assert.True(ts.Ts.T, status.IsStatusOK(), "Bad status: %v", status)
		}
	}
	db.DPrintf(db.TEST, "%v burst-spawned procs have all started", len(ps))
}

func evictProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	for _, p := range ps {
		err := ts.Evict(p.GetPid())
		assert.Nil(ts.Ts.T, err, "Evict: %v", err)
		status, err := ts.WaitExit(p.GetPid())
		assert.True(ts.Ts.T, status.IsStatusEvicted(), "Bad status evict: %v", status)
	}
}

func runDummySpawnBenchProc(ts *test.RealmTstate, sclnt *sigmaclnt.SigmaClnt, pid sp.Tpid, isLC bool) time.Duration {
	p := proc.NewProcPid(pid, sp.DUMMY_PROG, nil)
	if isLC {
		// Set a minimal amount of MCPU if spawning an LC proc
		p.SetMcpu(10)
	}
	err := sclnt.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Spawn: %v", err)
	status, err := sclnt.WaitExit(p.GetPid())
	if assert.Nil(ts.Ts.T, err, "WaitExit: %v", err) {
		assert.False(ts.Ts.T, status.IsStatusOK(), "Wrong status: %v", status)
	}
	return 99 * time.Second
}

func runRustSpawnBenchProc(ts *test.RealmTstate, sclnt *sigmaclnt.SigmaClnt, prog string, pid sp.Tpid, kernelpref []string) time.Duration {
	p := proc.NewProcPid(pid, prog, nil)
	p.SetKernels(kernelpref)
	err := sclnt.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Spawn: %v", err)
	status, err := sclnt.WaitExit(p.GetPid())
	if assert.Nil(ts.Ts.T, err, "WaitExit: %v", err) {
		assert.False(ts.Ts.T, status.IsStatusOK(), "Wrong status: %v", status)
	}
	return 99 * time.Second
}

func runSpawnBenchProc(ts *test.RealmTstate, sclnt *sigmaclnt.SigmaClnt, pid sp.Tpid, kernelpref []string) time.Duration {
	p := proc.NewProcPid(pid, "spawn-bench", nil)
	p.SetKernels(kernelpref)
	err := sclnt.Spawn(p)
	assert.Nil(ts.Ts.T, err, "WaitStart: %v", err)
	status, err := sclnt.WaitExit(p.GetPid())
	if assert.Nil(ts.Ts.T, err, "WaitStart: %v", err) {
		ok := assert.True(ts.Ts.T, status.IsStatusOK(), "Wrong status: %v", status)
		if ok {
			return time.Duration(status.Data().(float64))
		}
	}
	return 99 * time.Second
}

// ========== Realm Helpers ==========

// Count the number of cores in the cluster.
func countClusterCores(rootts *test.Tstate) int {
	// XXX For now, we assume all machines have the same number of cores.
	sts, err := rootts.GetDir(sp.BOOT)
	assert.Nil(rootts.T, err)

	ncores := len(sts) * int(linuxsched.GetNCores())
	db.DPrintf(db.TEST, "Aggregate number of cores in the cluster: %v", ncores)
	return ncores
}

// Block off physical memory on every machine
func blockMem(rootts *test.Tstate, mem string) []*proc.Proc {
	if mem == "0MB" {
		db.DPrintf(db.TEST, "No mem blocking")
		return nil
	}
	sdc := scheddclnt.NewScheddClnt(rootts.SigmaClnt.FsLib, sp.NOT_SET)
	// Get the number of schedds.
	n, err := sdc.Nschedd()
	if err != nil {
		db.DFatalf("Can't count nschedd: %v", err)
	}
	db.DFatalf("Memory blocking deprecated")
	ps := make([]*proc.Proc, 0, n)
	for i := 0; i < n; i++ {
		db.DPrintf(db.TEST, "Spawning memblock %v for %v of memory", i, mem)
		p := proc.NewProc("memblock", []string{mem})
		// Make it LC so it doesn't get swapped.
		p.SetType(proc.T_LC)
		err := rootts.Spawn(p)
		if !assert.Nil(rootts.T, err, "Error spawn: %v", err) {
			db.DFatalf("Can't spawn blockers: %v", err)
		}
		err = rootts.WaitStart(p.GetPid())
		assert.Nil(rootts.T, err, "Error waitstart: %v", err)
		if err != nil {
			db.DFatalf("Error waitstart blocker: %v", err)
		}
		ps = append(ps, p)
	}
	db.DPrintf(db.TEST, "Done spawning memblockers")
	return ps
}

func evictMemBlockers(ts *test.Tstate, ps []*proc.Proc) {
	for _, p := range ps {
		err := ts.Evict(p.GetPid())
		assert.Nil(ts.T, err, "Evict: %v", err)
		status, err := ts.WaitExit(p.GetPid())
		if err != nil || !status.IsStatusEvicted() {
			db.DFatalf("Err waitexit blockers: status %v err %v", status, err)
		}
	}
}

// Warm up a realm, by starting uprocds for it on all machines in the cluster.
func warmupRealm(ts *test.RealmTstate, progs []string) (time.Time, int) {
	sdc := scheddclnt.NewScheddClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
	// Get the list of schedds.
	sds, err := sdc.GetSchedds()
	assert.Nil(ts.Ts.T, err, "Get Schedds: %v", err)
	db.DPrintf(db.TEST, "Warm up realm %v for progs %v schedds %d %v", ts.GetRealm(), progs, len(sds), sds)
	start := time.Now()
	nDL := 0
	for _, kid := range sds {
		// Warm the cache for a binary
		for _, ptype := range []proc.Ttype{proc.T_LC, proc.T_BE} {
			for _, prog := range progs {
				err := sdc.WarmUprocd(kid, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), prog+"-v"+sp.Version, ts.Ts.ProcEnv().GetSigmaPath(), ptype)
				nDL++
				assert.Nil(ts.Ts.T, err, "WarmUprocd: %v", err)
			}
		}
	}
	db.DPrintf(db.TEST, "Warmed up realm %v", ts.GetRealm())
	return start, nDL
}

// ========== Dir Helpers ==========

func newOutDir(ts *test.RealmTstate) {
	err := ts.MkDir(OUT_DIR, 0777)
	assert.Nil(ts.Ts.T, err, "Couldn't make out dir: %v", err)
}

func rmOutDir(ts *test.RealmTstate) {
	err := ts.RmDir(OUT_DIR)
	assert.Nil(ts.Ts.T, err, "Couldn't rm out dir: %v", err)
}

// ========== Semaphore Helpers ==========

func newNSemaphores(ts *test.RealmTstate, n int) ([]*semclnt.SemClnt, []interface{}) {
	ss := make([]*semclnt.SemClnt, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		spath := filepath.Join(OUT_DIR, rand.String(16))
		s := semclnt.NewSemClnt(ts.FsLib, spath)
		ss = append(ss, s)
		is = append(is, s)
	}
	return ss, is
}

// ========== MR Helpers ========

func newNMRJobs(ts *test.RealmTstate, p *perf.Perf, n int, app string, jobRoot string, memreq proc.Tmem) ([]*MRJobInstance, []interface{}) {
	ms := make([]*MRJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := NewMRJobInstance(ts, p, app, jobRoot, app+"-mr-"+rand.String(3)+"-"+ts.GetRealm().String(), memreq)
		ms = append(ms, i)
		is = append(is, i)
	}
	return ms, is
}

// ========== KV Helpers ========

func parseDurations(ts *test.RealmTstate, ss []string) []time.Duration {
	ds := make([]time.Duration, 0, len(ss))
	for _, s := range ss {
		d, err := time.ParseDuration(s)
		assert.Nil(ts.Ts.T, err, "Error parse duration: %v", err)
		ds = append(ds, d)
	}
	return ds
}

func newNKVJobs(ts *test.RealmTstate, n, nkvd, kvdrepl int, nclerks []int, phases []time.Duration, ckdur string, kvdmcpu, ckmcpu proc.Tmcpu, auto string, redisaddr string) ([]*KVJobInstance, []interface{}) {
	// If we're running with unbounded clerks...
	if len(phases) > 0 {
		assert.Equal(ts.Ts.T, len(nclerks), len(phases), "Phase and clerk lengths don't match: %v != %v", len(phases), len(nclerks))
	}
	js := make([]*KVJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		ji := NewKVJobInstance(ts, nkvd, kvdrepl, nclerks, phases, ckdur, kvdmcpu, ckmcpu, auto, redisaddr)
		js = append(js, ji)
		is = append(is, ji)
	}
	return js, is
}

// ========== Cached Helpers ==========
func newNCachedJobs(ts *test.RealmTstate, n, nkeys, ncache, nclerks int, durstr string, ckmcpu, cachemcpu proc.Tmcpu) ([]*CachedJobInstance, []interface{}) {
	js := make([]*CachedJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	durs := parseDurations(ts, []string{durstr})
	for i := 0; i < n; i++ {
		ji := NewCachedJob(ts, nkeys, ncache, nclerks, durs[0], ckmcpu, cachemcpu)
		js = append(js, ji)
		is = append(is, ji)
	}
	return js, is
}

// ========== Schedd Helpers ==========

func newScheddJobs(ts *test.RealmTstate, nclnt int, dur string, maxrps string, progname string, sfn scheddFn, kernels []string, withKernelPref, skipstats bool) ([]*ScheddJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*ScheddJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := NewScheddJob(ts, nclnt, dur, maxrps, progname, sfn, kernels, withKernelPref, skipstats)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== Www Helpers ========

func newWwwJobs(ts *test.RealmTstate, sigmaos bool, n int, wwwmcpu proc.Tmcpu, reqtype string, ntrials, nclnt, nreq int, delay time.Duration) ([]*WwwJobInstance, []interface{}) {
	ws := make([]*WwwJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := NewWwwJob(ts, sigmaos, wwwmcpu, reqtype, ntrials, nclnt, nreq, delay)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== Hotel Helpers ==========

func newHotelJobs(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, dur string, maxrps string, ncache int, cachetype string, cacheMcpu proc.Tmcpu, manuallyScaleCaches bool, scaleCacheDelay time.Duration, nCachesToAdd int, nGeo int, manuallyScaleGeo bool, scaleGeoDelay time.Duration, nGeoToAdd int, fn hotelFn) ([]*HotelJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*HotelJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := NewHotelJob(ts, p, sigmaos, dur, maxrps, fn, false, ncache, cachetype, cacheMcpu, manuallyScaleCaches, scaleCacheDelay, nCachesToAdd, nGeo, manuallyScaleGeo, scaleGeoDelay, nGeoToAdd)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

func newHotelJobsCli(ts *test.RealmTstate, sigmaos bool, dur string, maxrps string, ncache int, cachetype string, cacheMcpu proc.Tmcpu, manuallyScaleCaches bool, scaleCacheDelay time.Duration, nCachesToAdd int, nGeo int, manuallyScaleGeo bool, scaleGeoDelay time.Duration, nGeoToAdd int, fn hotelFn) ([]*HotelJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*HotelJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := NewHotelJob(ts, nil, sigmaos, dur, maxrps, fn, true, ncache, cachetype, cacheMcpu, manuallyScaleCaches, scaleCacheDelay, nCachesToAdd, nGeo, manuallyScaleGeo, scaleGeoDelay, nGeoToAdd)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== ImgResize Helpers ==========
func newImgResizeJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, input string, ntasks int, ninputs int, mcpu proc.Tmcpu, mem proc.Tmem, nrounds int, imgdmcpu proc.Tmcpu) ([]*ImgResizeJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*ImgResizeJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := NewImgResizeJob(ts, p, sigmaos, input, ntasks, ninputs, mcpu, mem, nrounds, imgdmcpu)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== ImgResizeRPC Helpers ==========
func newImgResizeRPCJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, input string, tasksPerSec int, dur time.Duration, mcpu proc.Tmcpu, mem proc.Tmem, nrounds int, imgdmcpu proc.Tmcpu) ([]*ImgResizeRPCJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*ImgResizeRPCJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := NewImgResizeRPCJob(ts, p, sigmaos, input, tasksPerSec, dur, mcpu, mem, nrounds, imgdmcpu)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== Social Network Helpers ==========

func newSocialNetworkJobs(
	ts *test.RealmTstate, p *perf.Perf, sigmaos, readonly bool,
	dur, maxrps string, ncache int) ([]*SocialNetworkJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*SocialNetworkJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := NewSocialNetworkJob(ts, p, sigmaos, readonly, dur, maxrps, ncache)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== Client Helpers ==========

var clidir string = filepath.Join("name/", "clnts")

// Wait for a realm to be created
func waitForRealmCreation(rootts *test.Tstate, realm sp.Trealm) error {
	dirs := []string{
		"",
		sp.KPIDSREL,
		sp.S3REL,
		sp.UXREL,
	}
	for _, d := range dirs {
		if err := rootts.WaitCreate(filepath.Join(sp.REALMS, realm.String(), d)); err != nil {
			return err
		}
	}
	return nil
}

func createClntWaitSem(rootts *test.Tstate) *semclnt.SemClnt {
	sem := semclnt.NewSemClnt(rootts.FsLib, filepath.Join(clidir, "clisem"))
	err := sem.Init(0)
	if !assert.True(rootts.T, err == nil || !serr.IsErrCode(err, serr.TErrExists), "Error sem init %v", err) {
		return nil
	}
	db.DPrintf(db.TEST, "Create sem %v", sem)
	return sem
}

// Waits for n - 1 clients to mark themselves as ready, releases them, and then
// returns.
func waitForClnts(rootts *test.Tstate, n int) {
	// Make sure the clients directory has been created.
	err := rootts.MkDir(clidir, 0777)
	assert.True(rootts.T, err == nil || serr.IsErrCode(err, serr.TErrExists), "Error mkdir: %v", err)

	// Wait for n - 1 clnts to register themselves.
	dr := fslib.NewDirReader(rootts.FsLib, clidir)
	err = dr.WaitNEntries(n) // n - 1 + the semaphore
	assert.Nil(rootts.T, err, "Err WaitNentries: %v", err)
	sts, err := rootts.GetDir(clidir)
	assert.Nil(rootts.T, err, "Err GetDir: %v", err)
	db.DPrintf(db.TEST, "Got clients: %v", sp.Names(sts))
	sem := createClntWaitSem(rootts)
	err = sem.Up()
	assert.Nil(rootts.T, err, "Err sem.Up: %v", err)
}

// Marks client as ready, waits for leader to release clients, adn then
// returns.
func clientReady(rootts *test.Tstate) {
	// Make sure the clients directory has been created.
	err := rootts.MkDir(clidir, 0777)
	assert.True(rootts.T, err == nil || serr.IsErrCode(err, serr.TErrExists), "Error mkdir: %v", err)
	// Create a semaphore, which the leader will signal in order to start the benchmark
	sem := createClntWaitSem(rootts)
	// Register the client as ready.
	cid := "clnt-" + rand.String(4)
	_, err = rootts.PutFile(filepath.Join(clidir, cid), 0777, sp.OWRITE, nil)
	assert.Nil(rootts.T, err, "Err PutFile: %v", err)
	// Wait for the leader's signal
	db.DPrintf(db.TEST, "sem.Down %v", cid)
	sem.Down()
	db.DPrintf(db.TEST, "sem.Down done %v", cid)
}

// ========== Download Results Helpers ==========

func downloadS3Results(ts *test.Tstate, src string, dst string) {
	downloadS3ResultsRealm(ts, src, dst, "")
}

func downloadS3ResultsRealm(ts *test.Tstate, src string, dst string, realm sp.Trealm) {
	// Make the destination directory.
	os.MkdirAll(dst, 0777)
	_, err := ts.ProcessDir(src, func(st *sp.Stat) (bool, error) {
		rdr, err := ts.OpenReader(filepath.Join(src, st.Name))
		defer rdr.Close()
		assert.Nil(ts.T, err, "Error open reader %v", err)
		b, err := io.ReadAll(rdr)
		assert.Nil(ts.T, err, "Error read all %v", err)
		name := st.Name
		if realm.String() != "" {
			name += "-" + realm.String() + "-tpt.out"
		}
		err = os.WriteFile(filepath.Join(dst, name), b, 0777)
		assert.Nil(ts.T, err, "Error write file %v", err)
		return false, nil
	})
	assert.Nil(ts.T, err, "Error process dir %v", err)
}

// ========== Start/Wait K8s MR Helpers ==========

func startK8sMR(ts *test.Tstate, coordaddr string) *rpc.Client {
	c, err := rpc.DialHTTP("tcp", coordaddr)
	assert.Nil(ts.T, err, "Error dial coord: %v", err)
	var req bool
	var res bool
	err = c.Call("K8sCoord.Start", &req, &res)
	assert.Nil(ts.T, err, "Error Start coord: %v", err)
	return c
}

func waitK8sMR(ts *test.Tstate, c *rpc.Client) {
	var req bool
	var res bool
	err := c.Call("K8sCoord.WaitDone", &req, &res)
	assert.Nil(ts.T, err, "Error WaitDone coord: %v", err)
	time.Sleep(10 * time.Second)
}

func k8sMRAddr(k8sLeaderNodeIP string, port int) string {
	return k8sLeaderNodeIP + ":" + strconv.Itoa(port)
}
