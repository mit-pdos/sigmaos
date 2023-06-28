package benchmarks_test

import (
	"errors"
	"io"
	"net/rpc"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/scheddclnt"
	"sigmaos/semclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

//
// A set of helper functions that we use across our benchmarks.
//

// ========== Proc Helpers ==========

func makeNProcs(n int, prog string, args []string, env map[string]string, ncore proc.Tcore) ([]*proc.Proc, []interface{}) {
	ps := make([]*proc.Proc, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProc(prog, args)
		for k, v := range env {
			p.AppendEnv(k, v)
		}
		p.SetNcore(ncore)
		ps = append(ps, p)
		is = append(is, p)
	}
	return ps, is
}

func spawnBurstProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	db.DPrintf(db.TEST, "Burst-spawning %v procs in chunks of size %v", len(ps), len(ps)/MAX_PARALLEL)
	_, errs := ts.SpawnBurst(ps, 1)
	assert.Equal(ts.T, len(errs), 0, "Errors SpawnBurst: %v", errs)
}

func spawnProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	for _, p := range ps {
		err := ts.Spawn(p)
		assert.Nil(ts.T, err, "WaitStart: %v", err)
	}
}

func waitStartProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	for _, p := range ps {
		err := ts.WaitStart(p.GetPid())
		assert.Nil(ts.T, err, "WaitStart: %v", err)
	}
	db.DPrintf(db.TEST, "%v burst-spawned procs have all started", len(ps))
}

func waitExitProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	for _, p := range ps {
		status, err := ts.WaitExit(p.GetPid())
		assert.Nil(ts.T, err, "WaitStart: %v", err)
		assert.True(ts.T, status.IsStatusOK(), "Bad status: %v", status)
	}
	db.DPrintf(db.TEST, "%v burst-spawned procs have all started", len(ps))
}

func evictProcs(ts *test.RealmTstate, ps []*proc.Proc) {
	for _, p := range ps {
		err := ts.Evict(p.GetPid())
		assert.Nil(ts.T, err, "Evict: %v", err)
		status, err := ts.WaitExit(p.GetPid())
		assert.True(ts.T, status.IsStatusEvicted(), "Bad status evict: %v", status)
	}
}

// ========== Realm Helpers ==========

// Count the number of cores in the cluster.
func countClusterCores(rootts *test.Tstate) proc.Tcore {
	// XXX For now, we assume all machines have the same number of cores.
	sts, err := rootts.GetDir(sp.BOOT)
	assert.Nil(rootts.T, err)

	ncores := proc.Tcore(len(sts) * int(linuxsched.NCores))
	db.DPrintf(db.TEST, "Aggregate number of cores in the cluster: %v", ncores)
	return ncores
}

// Block off physical memory on every machine
func blockMem(rootts *test.Tstate, mem string) []*proc.Proc {
	if mem == "0MB" {
		db.DPrintf(db.TEST, "No mem blocking")
		return nil
	}
	sdc := scheddclnt.MakeScheddClnt(rootts.SigmaClnt.FsLib, sp.ROOTREALM)
	// Get the number of schedds.
	n, err := sdc.Nschedd()
	if err != nil {
		db.DFatalf("Can't count nschedd: %v", err)
	}
	ps := make([]*proc.Proc, 0, n)
	for i := 0; i < n; i++ {
		db.DPrintf(db.TEST, "Spawning memblock %v for %v of memory", i, mem)
		p := proc.MakeProc("memblock", []string{mem})
		// Make it LC so it doesn't get swapped.
		p.SetType(proc.T_LC)
		_, errs := rootts.SpawnBurst([]*proc.Proc{p}, 1)
		assert.True(rootts.T, len(errs) == 0, "Error spawn: %v", errs)
		if len(errs) > 0 {
			db.DFatalf("Can't spawn blockers: %v", err)
		}
		err := rootts.WaitStart(p.GetPid())
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
func warmupRealm(ts *test.RealmTstate) {
	sdc := scheddclnt.MakeScheddClnt(ts.SigmaClnt.FsLib, ts.GetRealm())
	// Get the number of schedds.
	n, err := sdc.Nschedd()
	assert.Nil(ts.T, err, "Get NSchedd: %v", err)
	// Spawn one BE and one LC proc on each schedd, to force uprocds to start.
	for _, ncore := range []proc.Tcore{0, 1} {
		// Make N LC procs.
		ps, _ := makeNProcs(n, "sleeper", []string{"1000us", ""}, nil, ncore)
		// Burst the procs across the available schedds.
		spawnBurstProcs(ts, ps)
		// Wait for them to exit.
		waitExitProcs(ts, ps)
	}
	db.DPrintf(db.TEST, "Warmed up realm %v", ts.GetRealm())
}

// ========== Dir Helpers ==========

func makeOutDir(ts *test.RealmTstate) {
	err := ts.MkDir(OUT_DIR, 0777)
	assert.Nil(ts.T, err, "Couldn't make out dir: %v", err)
}

func rmOutDir(ts *test.RealmTstate) {
	err := ts.RmDir(OUT_DIR)
	assert.Nil(ts.T, err, "Couldn't rm out dir: %v", err)
}

// ========== Semaphore Helpers ==========

func makeNSemaphores(ts *test.RealmTstate, n int) ([]*semclnt.SemClnt, []interface{}) {
	ss := make([]*semclnt.SemClnt, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		spath := path.Join(OUT_DIR, rand.String(16))
		s := semclnt.MakeSemClnt(ts.FsLib, spath)
		ss = append(ss, s)
		is = append(is, s)
	}
	return ss, is
}

// ========== MR Helpers ========

func makeNMRJobs(ts *test.RealmTstate, p *perf.Perf, n int, app string) ([]*MRJobInstance, []interface{}) {
	ms := make([]*MRJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeMRJobInstance(ts, p, app, app+"-mr-"+rand.String(16)+"-"+ts.GetRealm().String())
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
		assert.Nil(ts.T, err, "Error parse duration: %v", err)
		ds = append(ds, d)
	}
	return ds
}

func makeNKVJobs(ts *test.RealmTstate, n, nkvd, kvdrepl int, nclerks []int, phases []time.Duration, ckdur string, kvdncore, ckncore proc.Tcore, auto string, redisaddr string) ([]*KVJobInstance, []interface{}) {
	// If we're running with unbounded clerks...
	if len(phases) > 0 {
		assert.Equal(ts.T, len(nclerks), len(phases), "Phase and clerk lengths don't match: %v != %v", len(phases), len(nclerks))
	}
	js := make([]*KVJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		ji := MakeKVJobInstance(ts, nkvd, kvdrepl, nclerks, phases, ckdur, kvdncore, ckncore, auto, redisaddr)
		js = append(js, ji)
		is = append(is, ji)
	}
	return js, is
}

// ========== Cached Helpers ==========
func makeNCachedJobs(ts *test.RealmTstate, n, nkeys, ncache, nclerks int, durstr string, ckncore, cachencore proc.Tcore) ([]*CachedJobInstance, []interface{}) {
	js := make([]*CachedJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	durs := parseDurations(ts, []string{durstr})
	for i := 0; i < n; i++ {
		ji := MakeCachedJob(ts, nkeys, ncache, nclerks, durs[0], ckncore, cachencore)
		js = append(js, ji)
		is = append(is, ji)
	}
	return js, is
}

// ========== Www Helpers ========

func makeWwwJobs(ts *test.RealmTstate, sigmaos bool, n int, wwwncore proc.Tcore, reqtype string, ntrials, nclnt, nreq int, delay time.Duration) ([]*WwwJobInstance, []interface{}) {
	ws := make([]*WwwJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeWwwJob(ts, sigmaos, wwwncore, reqtype, ntrials, nclnt, nreq, delay)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== RPCBench Helpers ========

func makeRPCBenchJobs(ts *test.RealmTstate, p *perf.Perf, ncore proc.Tcore, dur string, maxrps string, fn rpcbenchFn) ([]*RPCBenchJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*RPCBenchJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeRPCBenchJob(ts, p, ncore, dur, maxrps, fn, false)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

func makeRPCBenchJobsCli(ts *test.RealmTstate, p *perf.Perf, ncore proc.Tcore, dur string, maxrps string, fn rpcbenchFn) ([]*RPCBenchJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*RPCBenchJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeRPCBenchJob(ts, p, ncore, dur, maxrps, fn, true)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== Hotel Helpers ==========

func makeHotelJobs(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, dur string, maxrps string, ncache int, cachetype string, cacheNcore proc.Tcore, fn hotelFn) ([]*HotelJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*HotelJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeHotelJob(ts, p, sigmaos, dur, maxrps, fn, false, ncache, cachetype, cacheNcore)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

func makeHotelJobsCli(ts *test.RealmTstate, sigmaos bool, dur string, maxrps string, ncache int, cachetype string, cacheNcore proc.Tcore, fn hotelFn) ([]*HotelJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*HotelJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeHotelJob(ts, nil, sigmaos, dur, maxrps, fn, true, ncache, cachetype, cacheNcore)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== ImgResize Helpers ==========
func makeImgResizeJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, input string, ntasks int) ([]*ImgResizeJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*ImgResizeJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeImgResizeJob(ts, p, sigmaos, input, ntasks)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== Client Helpers ==========

var clidir string = path.Join("name/", "clnts")

func createClntWaitSem(rootts *test.Tstate) *semclnt.SemClnt {
	sem := semclnt.MakeSemClnt(rootts.FsLib, path.Join(clidir, "clisem"))
	var serr *serr.Err
	err := sem.Init(0)
	if !assert.True(rootts.T, err == nil || errors.As(err, &serr) && !serr.IsErrExists(), "Error sem init %v", err) {
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
	var serr *serr.Err
	assert.True(rootts.T, err == nil || errors.As(err, &serr) && serr.IsErrExists(), "Error mkdir: %v", err)
	// Wait for n - 1 clnts to register themselves.
	_, err = rootts.ReadDirWatch(clidir, func(sts []*sp.Stat) bool {
		db.DPrintf(db.TEST, "%v clients ready %v", len(sts), sp.Names(sts))
		// N - 1 clnts + the semaphore
		return len(sts) < n
	})
	assert.Nil(rootts.T, err, "Err ReadDirWatch: %v", err)
	sem := createClntWaitSem(rootts)
	err = sem.Up()
	assert.Nil(rootts.T, err, "Err sem.Up: %v", err)
}

// Marks client as ready, waits for leader to release clients, adn then
// returns.
func clientReady(rootts *test.Tstate) {
	// Make sure the clients directory has been created.
	err := rootts.MkDir(clidir, 0777)
	var serr *serr.Err
	assert.True(rootts.T, err == nil || errors.As(err, &serr) && serr.IsErrExists(), "Error mkdir: %v", err)
	// Register the client as ready.
	cid := "clnt-" + rand.String(4)
	_, err = rootts.PutFile(path.Join(clidir, cid), 0777, sp.OWRITE, nil)
	assert.Nil(rootts.T, err, "Err PutFile: %v", err)
	// Create a semaphore and wait for the leader to start the benchmark
	sem := createClntWaitSem(rootts)
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
		rdr, err := ts.OpenReader(path.Join(src, st.Name))
		defer rdr.Close()
		assert.Nil(ts.T, err, "Error open reader %v", err)
		b, err := io.ReadAll(rdr)
		assert.Nil(ts.T, err, "Error read all %v", err)
		name := st.Name
		if realm.String() != "" {
			name += "-" + realm.String() + "-tpt.out"
		}
		err = os.WriteFile(path.Join(dst, name), b, 0777)
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
