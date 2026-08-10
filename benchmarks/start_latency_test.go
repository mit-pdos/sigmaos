package benchmarks_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cacheproto "sigmaos/apps/cache/proto"
	cossimclnt "sigmaos/apps/cossim/clnt"
	cossimsrv "sigmaos/apps/cossim/srv"
	epsrv "sigmaos/apps/epcache/srv"
	"sigmaos/apps/etcd"
	imgrec_py "sigmaos/apps/imgrec/py"
	imgrec_wasm "sigmaos/apps/imgrec/wasm"
	"sigmaos/apps/memcached"
	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/proc"
	s3clnt "sigmaos/proxy/s3/clnt"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sched/msched/proc/chunk"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type StartLatencyJobInstance struct {
	*test.RealmTstate
	jobName       string
	cfg           *benchmarks.StartLatencyBenchConfig
	cacheCfg      *benchmarks.CacheBenchConfig
	cossimCfg     *benchmarks.CosSimBenchConfig
	etcdCfg       *benchmarks.EtcdBenchConfig
	memcachedCfg  *benchmarks.MemcachedBenchConfig
	imgrecPyCfg   *benchmarks.ImgrecPyBenchConfig
	imgrecWASMCfg *benchmarks.ImgrecWASMBenchConfig
	ready         chan bool
	epcj          *epsrv.EPCacheJob
	cacheJob      *cachegrpmgr.CacheMgr
	msc           *mschedclnt.MSchedClnt
	cacheClnt     *cachegrpclnt.CachedSvcClnt
	cossimJob     *cossimsrv.CosSimJob
	cossimClnt    *cossimclnt.CosSimShardClnt
	etcdJob       *etcd.EtcdJob
	memcachedJob  *memcached.MemcachedJob
	imgrecPyJob   *imgrec_py.ImgrecPyJob
	imgrecWASMJob *imgrec_wasm.ImgrecWASMJob
	sleeperProc   *proc.Proc
	keys          []string
	vals          []*cacheproto.CacheString
	needCached    bool
	warmSrvKID    string
}

func NewStartLatencyJob(ts *test.RealmTstate, cfg *benchmarks.StartLatencyBenchConfig, cacheCfg *benchmarks.CacheBenchConfig, cossimCfg *benchmarks.CosSimBenchConfig, etcdCfg *benchmarks.EtcdBenchConfig, memcachedCfg *benchmarks.MemcachedBenchConfig, imgrecPyCfg *benchmarks.ImgrecPyBenchConfig, imgrecWASMCfg *benchmarks.ImgrecWASMBenchConfig) *StartLatencyJobInstance {
	ji := &StartLatencyJobInstance{
		RealmTstate:   ts,
		jobName:       "start-latency",
		cfg:           cfg,
		cacheCfg:      cacheCfg,
		cossimCfg:     cossimCfg,
		etcdCfg:       etcdCfg,
		memcachedCfg:  memcachedCfg,
		imgrecPyCfg:   imgrecPyCfg,
		imgrecWASMCfg: imgrecWASMCfg,
		ready:         make(chan bool),
	}
	ji.msc = mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
	// Create a sleeper proc, which will block off a machine which can be used to
	// host binaries in the cluster
	ji.sleeperProc = proc.NewProc("sleeper", []string{fmt.Sprintf("%v", 1000*time.Second), "name/"})
	ji.sleeperProc.SetMcpu(4000)
	err := ji.Spawn(ji.sleeperProc)
	if !assert.Nil(ts.Ts.T, err, "Err Spawn sleeper proc: %v", err) {
		return ji
	}
	err = ji.WaitStart(ji.sleeperProc.GetPid())
	if !assert.Nil(ts.Ts.T, err, "Err WaitStart sleeper proc: %v", err) {
		return ji
	}
	foundSleeper := false
	runningProcs, err := ji.msc.GetAllRunningProcs()
	if !assert.Nil(ts.Ts.T, err, "Err GetRunningProcs: %v", err) {
		return ji
	}
	for _, p := range runningProcs[ts.GetRealm()] {
		// Record where relevant programs are running
		switch p.GetProgram() {
		case "sleeper":
			ji.warmSrvKID = p.GetKernelID()
			db.DPrintf(db.TEST, "sleeper[%v] running on kernel %v", p.GetPid(), p.GetKernelID())
			foundSleeper = true
		default:
		}
	}
	if !assert.True(ts.Ts.T, foundSleeper, "Err didn't find sleeper for warm srv") {
		return ji
	}
	// Warm up an msched currently running the sleeper with the binaries for the
	// other procs.  No proc will be able to actually run on this machine (the
	// CPU reservation conflicts with that of the other procs)
	bins := []string{
		"cossim-srv-cpp-v" + sp.Version,
		"cached-srv-cpp-v" + sp.Version,
		"etcd-v" + sp.Version,
		"etcd-shim-v" + sp.Version,
		"memcached-v" + sp.Version,
		"memcached-shim-v" + sp.Version,
		"imgrec.py-v" + sp.Version,
		"imgrec_precompiled.wasm-v" + sp.Version,
	}
	if err = precompileUploadWasmProcs(ts); !assert.Nil(ts.Ts.T, err, "Err precompile/upload WASM procs: %v", err) {
		return ji
	}
	// Warm up the warm server with the proc binaries
	for _, bin := range bins {
		db.DPrintf(db.TEST, "Target kernel to run prewarm with %v bin: %v", bin, ji.warmSrvKID)
		err = ji.msc.WarmProcd(ji.warmSrvKID, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), bin, ts.Ts.ProcEnv().GetSigmaPath(), proc.T_LC, "")
		if !assert.Nil(ts.Ts.T, err, "Err warming third msched with cossim bin: %v", err) {
			return ji
		}
		db.DPrintf(db.TEST, "Warmed kid %v with %v bin", ji.warmSrvKID, bin)
	}
	// Create EPCache
	ji.epcj, err = epsrv.NewEPCacheJob(ts.SigmaClnt)
	if !assert.Nil(ji.Ts.T, err, "Err new epCacheJob: %v", err) {
		return ji
	}
	ji.needCached = ji.cfg.App == "cached" || ji.cfg.App == "cossim"
	if ji.needCached {
		// Create cache manager
		ji.cacheJob, err = cachegrpmgr.NewCacheMgrEPCache(ts.SigmaClnt, ji.epcj, ji.jobName, ji.cacheCfg.JobCfg)
		if !assert.Nil(ts.Ts.T, err, "Err new cachemgr: %v", err) {
			return ji
		}
		// Create cache client
		if ji.cacheCfg.UseEPCache {
			ji.cacheClnt = cachegrpclnt.NewCachedSvcClntEPCache(ts.FsLib, ji.epcj.Clnt, ji.jobName)
		} else {
			ji.cacheClnt = cachegrpclnt.NewCachedSvcClnt(ts.FsLib, ji.jobName)
		}
	}
	switch ji.cfg.App {
	case "cached":
		// If running cached bench, write some KVs to the cache
		ji.keys, ji.vals, err = ji.writeKVsToCache()
		if !assert.Nil(ji.Ts.T, err, "Err write KVs to cache: %v", err) {
			return ji
		}
	case "cossim":
		// Create cossim job
		ji.cossimJob, err = cossimsrv.NewCosSimJob(ji.cossimCfg.JobCfg, ts.SigmaClnt, ji.epcj, ji.cacheJob, ji.cacheClnt)
		if !assert.Nil(ts.Ts.T, err, "Err new cossim job: %v", err) {
			return ji
		}
		ji.cossimClnt = ji.cossimJob.Clnt
	case "etcd":
		// Create etcd job
		ji.etcdJob, err = etcd.NewEtcdJob(ji.etcdCfg.JobCfg, ts.SigmaClnt, ji.epcj)
		if !assert.Nil(ts.Ts.T, err, "Err new etcd job: %v", err) {
			return ji
		}
		if ji.etcdCfg.JobCfg.UseUX {
			srcPath, _ := strings.CutPrefix(ji.etcdCfg.JobCfg.SnapshotS3Path, sp.S3+sp.LOCAL+"/")
			dstPath, _ := strings.CutPrefix(ji.etcdCfg.JobCfg.SnapshotUXPath, sp.UX+sp.LOCAL+"/")
			err = ji.etcdJob.DownloadSnapToAllUXs(srcPath, dstPath)
			if !assert.Nil(ts.Ts.T, err, "Err DownloadSnapToAllUXs etcd: %v", err) {
				return ji
			}
		}
	case "imgrec-py":
		ji.imgrecPyJob, err = imgrec_py.NewImgrecPyJob(ji.imgrecPyCfg.JobCfg, ts.SigmaClnt)
		if !assert.Nil(ts.Ts.T, err, "Err new imgrec-py job: %v", err) {
			return ji
		}
	case "imgrec-wasm":
		ji.imgrecWASMJob, err = imgrec_wasm.NewImgrecWASMJob(ji.imgrecWASMCfg.JobCfg, ts.SigmaClnt)
		if !assert.Nil(ts.Ts.T, err, "Err new imgrec-wasm job: %v", err) {
			return ji
		}
	case "memcached":
		// Create memcached job
		ji.memcachedJob, err = memcached.NewMemcachedJob(ji.memcachedCfg.JobCfg, ts.SigmaClnt, ji.epcj)
		if !assert.Nil(ts.Ts.T, err, "Err new memcached job: %v", err) {
			return ji
		}
		if ji.memcachedCfg.JobCfg.UseUX {
			// Download snapshot from S3 to all UX servers
			srcPath, _ := strings.CutPrefix(ji.memcachedCfg.JobCfg.SnapshotS3Path, sp.S3+sp.LOCAL+"/")
			dstPath, _ := strings.CutPrefix(ji.memcachedCfg.JobCfg.SnapshotUXPath, sp.UX+sp.LOCAL+"/")
			err = ji.memcachedJob.DownloadSnapToAllUXs(srcPath, dstPath)
			if !assert.Nil(ts.Ts.T, err, "Err DownloadSnapToAllUXs: %v", err) {
				return ji
			}
		} else if ji.memcachedCfg.Cache {
			// S3 path: force each proxy to cache the snapshot in-memory
			sts, err := ji.GetDir(sp.S3)
			if !assert.Nil(ji.Ts.T, err, "Err GetDir S3: %v", err) {
				return ji
			}
			relPath, _ := strings.CutPrefix(ji.memcachedCfg.JobCfg.SnapshotS3Path, sp.S3+sp.LOCAL+"/")
			pn := strings.Split(relPath, "/")
			bucket := pn[0]
			key := filepath.Join(pn[1:]...)
			for _, st := range sp.Names(sts) {
				s3clnt, err := s3clnt.NewS3Clnt(ji.FsLib, filepath.Join(sp.S3, st))
				if !assert.Nil(ji.Ts.T, err, "Err NewS3Clnt for %v: %v", st, err) {
					return ji
				}
				db.DPrintf(db.TEST, "Force caching of %v %v on %v", bucket, key, st)
				if _, err := s3clnt.GetObject(bucket, key, true); !assert.Nil(ji.Ts.T, err, "GetObject[%v] cache: %v", st, err) {
					return ji
				}
			}
		}
	}
	return ji
}

func (ji *StartLatencyJobInstance) RunJob(rs *benchmarks.Results, crash bool) bool {
	// TODO: get pid of started procs & print it
	switch ji.cfg.App {
	case "cached":
		// Add a cached server
		err := ji.cacheJob.AddScalerServerWithSigmaPath(chunk.ChunkdPath(ji.warmSrvKID), ji.cacheCfg.UseCoSandbox, ji.cacheCfg.CPP, ji.cacheCfg.Shmem)
		if !assert.Nil(ji.Ts.T, err, "Err add cached srv: %v", err) {
			return false
		}
		db.DPrintf(db.BENCH, "Cached server started")
	case "cossim":
		// Add a cossim server
		_, _, err := ji.cossimJob.AddSrvWithSigmaPath(chunk.ChunkdPath(ji.warmSrvKID))
		if !assert.Nil(ji.Ts.T, err, "Err add cossim srv: %v", err) {
			return false
		}
		db.DPrintf(db.BENCH, "Cossim server started")
	case "imgrec-py":
		if msg, err := ji.imgrecPyJob.Run(chunk.ChunkdPath(ji.warmSrvKID)); !assert.Nil(ji.Ts.T, err, "Err run imgrec-py: %v", err) {
			return false
		} else {
			// Rough check that the benchmark succeeded
			assert.True(ji.Ts.T, strings.Contains(msg, ","))
			db.DPrintf(db.BENCH, "imgrec-py ran. Result: %v", msg)
		}
	case "imgrec-wasm":
		if msg, err := ji.imgrecWASMJob.Run(chunk.ChunkdPath(ji.warmSrvKID)); !assert.Nil(ji.Ts.T, err, "Err run imgrec-wasm: %v", err) {
			return false
		} else {
			// Rough check that the benchmark succeeded
			assert.True(ji.Ts.T, strings.Contains(msg, ","))
			db.DPrintf(db.BENCH, "imgrec-wasm ran. Result: %v", msg)
		}
	case "etcd":
		// Start etcd
		if err := ji.etcdJob.Start(chunk.ChunkdPath(ji.warmSrvKID)); !assert.Nil(ji.Ts.T, err, "Err start etcd: %v", err) {
			return false
		}
		db.DPrintf(db.BENCH, "Etcd started")
	case "memcached":
		// Start memcached
		if err := ji.memcachedJob.Start(chunk.ChunkdPath(ji.warmSrvKID)); !assert.Nil(ji.Ts.T, err, "Err start memcached: %v", err) {
			return false
		}
		db.DPrintf(db.BENCH, "Memcached started")
	}
	return true
}

func (ji *StartLatencyJobInstance) Cleanup() {
	err := ji.Evict(ji.sleeperProc.GetPid())
	assert.Nil(ji.Ts.T, err, "Spawn")
	db.DPrintf(db.TEST, "Evicted sleeper")

	db.DPrintf(db.TEST, "Pre waitexit")
	status, err := ji.WaitExit(ji.sleeperProc.GetPid())
	db.DPrintf(db.TEST, "Post waitexit")
	assert.Nil(ji.Ts.T, err, "WaitExit error")
	assert.True(ji.Ts.T, status != nil && status.IsStatusEvicted(), "Exit status wrong: %v", status)

	switch ji.cfg.App {
	case "cossim":
		if ji.cossimJob != nil {
			ji.cossimJob.Stop()
		}
	case "etcd":
		if ji.etcdJob != nil {
			ji.etcdJob.Stop()
		}
	case "memcached":
		if ji.memcachedJob != nil {
			ji.memcachedJob.Stop()
		}
	}
	if ji.needCached {
		if ji.cacheJob != nil {
			ji.cacheJob.Stop()
		}
		if ji.epcj != nil {
			ji.epcj.Stop()
		}
	}
}

func precompileUploadWasmProcs(ts *test.RealmTstate) error {
	return imgrec_wasm.PrecompileAndUpload(ts.SigmaClnt)
}

// Write KVs to cache srv
func (ji *StartLatencyJobInstance) writeKVsToCache() ([]string, []*cacheproto.CacheString, error) {
	keys := make([]string, ji.cacheCfg.NKeys)
	vals := make([]*cacheproto.CacheString, ji.cacheCfg.NKeys)
	val := ""
	for i := 0; i < ji.cacheCfg.ValSize; i++ {
		val += "a"
	}
	for i := range keys {
		key := "key-" + strconv.Itoa(i)
		v := &cacheproto.CacheString{Val: val}
		keys[i] = key
		vals[i] = v
		if err := ji.cacheClnt.Put(key, v); err != nil {
			return nil, nil, err
		}
	}
	db.DPrintf(db.TEST, "Done write KVs to cache")
	return keys, vals, nil
}
