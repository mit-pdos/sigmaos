package benchmarks_test

import (
	"io"
	"net/rpc"
	"os"
	"path"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/semclnt"
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
	_, errs := ts.SpawnBurstParallel(ps, len(ps)/MAX_PARALLEL)
	assert.Equal(ts.T, len(errs), 0, "Errors SpawnBurst: %v", errs)
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

// Potentially pregrow a realm to encompass all cluster resources.
//func maybePregrowRealm(ts *test.RealmTstate) {
//	if PREGROW_REALM {
//		// Make sure we've counted the number of cores in the cluster.
//		countNClusterCores(ts)
//		fsl, err := fslib.MakeFsLib("test-rclnt")
//		assert.Nil(ts.T, err)
//		rclnt := realm.MakeRealmClntFsl(fsl, ts.ProcClnt)
//		// While we are missing cores, try to grow.
//		for realm.GetRealmConfig(rclnt.FsLib, ts.RealmId()).NCores != proc.Tcore(N_CLUSTER_CORES) {
//			rclnt.GrowRealm(ts.RealmId())
//		}
//		// Sleep for a bit, so procclnts will take note of the change.
//		time.Sleep(2 * sp.Conf.Realm.RESIZE_INTERVAL)
//		pdc := procdclnt.MakeProcdClnt(ts.FsLib, ts.RealmId())
//		n, _, err := pdc.Nprocd()
//		assert.Nil(ts.T, err, "Err %v", err)
//		db.DPrintf(db.TEST, "Pre-grew realm, now running with %v procds", n)
//	}
//}

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

func makeNMRJobs(ts *test.RealmTstate, n int, app string) ([]*MRJobInstance, []interface{}) {
	ms := make([]*MRJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeMRJobInstance(ts, app, app+"-mr-"+rand.String(16)+"-"+ts.GetRealm().String())
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

// ========== Hotel Helpers ========

func makeHotelJobs(ts *test.RealmTstate, sigmaos bool, dur string, maxrps string, fn hotelFn) ([]*HotelJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*HotelJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeHotelJob(ts, sigmaos, dur, maxrps, fn, false)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

func makeHotelJobsCli(ts *test.RealmTstate, sigmaos bool, dur string, maxrps string, fn hotelFn) ([]*HotelJobInstance, []interface{}) {
	// n is ntrials, which is always 1.
	n := 1
	ws := make([]*HotelJobInstance, 0, n)
	is := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		i := MakeHotelJob(ts, sigmaos, dur, maxrps, fn, true)
		ws = append(ws, i)
		is = append(is, i)
	}
	return ws, is
}

// ========== Download Results Helpers ==========

// downloadS3Results(ts , path.Join("name/s3/~any/9ps3/", outdir), test.HOSTTMP+ "perf-output")
func downloadS3Results(ts *test.Tstate, src string, dst string) {
	// Make the destination directory.
	os.MkdirAll(dst, 0777)
	_, err := ts.ProcessDir(src, func(st *sp.Stat) (bool, error) {
		rdr, err := ts.OpenReader(path.Join(src, st.Name))
		defer rdr.Close()
		assert.Nil(ts.T, err, "Error open reader %v", err)
		b, err := io.ReadAll(rdr)
		assert.Nil(ts.T, err, "Error read all %v", err)
		err = os.WriteFile(path.Join(dst, st.Name), b, 0777)
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
