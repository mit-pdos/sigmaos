package benchmarks_test

import (
	"math/rand"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cossimsrv "sigmaos/apps/cossim/srv"
	epsrv "sigmaos/apps/epcache/srv"
	"sigmaos/benchmarks"
	"sigmaos/benchmarks/loadgen"
	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sched/msched/proc/chunk"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/perf"
)

type cosSimFn func(j *cossimsrv.CosSimJob, r *rand.Rand)

type CosSimJobInstance struct {
	sigmaos          bool
	justCli          bool
	job              string
	fn               cosSimFn
	cfg              *benchmarks.CosSimBenchConfig
	ready            chan bool
	j                *cossimsrv.CosSimJob
	lgs              []*loadgen.LoadGenerator
	p                *perf.Perf
	msc              *mschedclnt.MSchedClnt
	cossimKIDs       map[string]bool
	cacheKIDs        map[string]bool
	warmCossimSrvKID string
	*test.RealmTstate
}

func NewCosSimJob(ts *test.RealmTstate, p *perf.Perf, epcj *epsrv.EPCacheJob, cm *cachegrpmgr.CacheMgr, cc *cachegrpclnt.CachedSvcClnt, sigmaos bool, fn cosSimFn, justCli bool, cfg *benchmarks.CosSimBenchConfig) *CosSimJobInstance {
	ji := &CosSimJobInstance{}
	ji.sigmaos = true
	ji.job = "cossim-job"
	ji.ready = make(chan bool)
	ji.fn = fn
	ji.RealmTstate = ts
	ji.p = p
	ji.justCli = justCli
	ji.cfg = cfg
	ji.cossimKIDs = make(map[string]bool)
	ji.cacheKIDs = make(map[string]bool)

	var err error

	if !ji.justCli {
		db.DPrintf(db.TEST, "Create new CosSim job")
		// Only start one cache if autoscaling.
		ji.j, err = cossimsrv.NewCosSimJob(ji.cfg.GetJobConfig(), ts.SigmaClnt, epcj, cm, cc)
		assert.Nil(ts.Ts.T, err, "Error NewCosSimJob: %v", err)
		db.DPrintf(db.TEST, "New CosSim job created")
		for i := 0; i < ji.cfg.JobCfg.InitNSrv; i++ {
			db.DPrintf(db.TEST, "Add initial cossim server %v", i)
			_, _, err := ji.j.AddSrv()
			if !assert.Nil(ts.Ts.T, err, "Err AddSrv: %v", err) {
				return ji
			}
			db.DPrintf(db.TEST, "Cossim server ready %v", i)
		}
		ji.msc = mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
		// Find machines were caches are running, and machines where the CosSim
		// server is running
		_, err := ji.msc.GetMScheds()
		if !assert.Nil(ts.Ts.T, err, "Err GetMScheds: %v", err) {
			return ji
		}
		foundCossim := false
		foundCached := false
		runningProcs, err := ji.msc.GetAllRunningProcs()
		if !assert.Nil(ts.Ts.T, err, "Err GetRunningProcs: %v", err) {
			return ji
		}
		for _, p := range runningProcs[ts.GetRealm()] {
			// Record where relevant programs are running
			switch p.GetProgram() {
			case "cossim-srv-cpp":
				ji.cossimKIDs[p.GetKernelID()] = true
				db.DPrintf(db.TEST, "cossim-srv-cpp[%v] running on kernel %v", p.GetPid(), p.GetKernelID())
				foundCossim = true
			case "cached":
				ji.cacheKIDs[p.GetKernelID()] = true
				ji.warmCossimSrvKID = p.GetKernelID()
				db.DPrintf(db.TEST, "cached[%v] running on kernel %v", p.GetPid(), p.GetKernelID())
				foundCached = true
			default:
			}
		}
		if !assert.True(ts.Ts.T, foundCossim, "Err didn't find cossim srv") {
			return ji
		}
		if !assert.True(ts.Ts.T, foundCached, "Err didn't find cached srv") {
			return ji
		}
		// Warm up an msched currently running a cached shard with the cossim srv
		// bin. No cossim server will be able to actually run on this machine (the
		// CPU reservation conflicts with that of the cached server), so we can be
		// sure that future servers which try to download the cossim srver binary
		// from this msched won't have to contend with the CPU utilization of an
		// existing cossim server under load.
		db.DPrintf(db.TEST, "Target kernel to run prewarm with CossimSrv bin: %v", ji.warmCossimSrvKID)
		err = ji.msc.WarmProcd(ji.warmCossimSrvKID, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), "cossim-srv-cpp-v"+sp.Version, ts.Ts.ProcEnv().GetSigmaPath(), proc.T_LC)
		if !assert.Nil(ts.Ts.T, err, "Err warming third msched with cossim bin: %v", err) {
			return ji
		}
		db.DPrintf(db.TEST, "Warmed kid %v with CossimSrv bin", ji.warmCossimSrvKID)
	}

	// Make a load generators.
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.cfg.Durs))
	for i := range ji.cfg.Durs {
		ji.lgs = append(ji.lgs, loadgen.NewLoadGenerator(ji.cfg.Durs[i], ji.cfg.MaxRPS[i], func(r *rand.Rand) (time.Duration, bool) {
			// Run a single request.
			ji.fn(ji.j, r)
			return 0, false
		}))
	}
	return ji
}

func (ji *CosSimJobInstance) StartCosSimJob() {
	db.DPrintf(db.ALWAYS, "StartCosSimJob CossimBenchCfg:%v", ji.cfg)
	var wg sync.WaitGroup
	for _, lg := range ji.lgs {
		wg.Add(1)
		go func(lg *loadgen.LoadGenerator, wg *sync.WaitGroup) {
			defer wg.Done()
			lg.Calibrate()
		}(lg, &wg)
	}
	wg.Wait()
	if !ji.justCli && ji.cfg.Scale.GetShouldScale() {
		go func() {
			time.Sleep(ji.cfg.Scale.GetScalingDelay())
			for i := 0; i < ji.cfg.Scale.GetNToAdd(); i++ {
				db.DPrintf(db.TEST, "Scale up cossim srvs to: %v", (i+1)+ji.cfg.JobCfg.InitNSrv)
				_, _, err := ji.j.AddSrvWithSigmaPath(chunk.ChunkdPath(ji.warmCossimSrvKID))
				assert.Nil(ji.Ts.T, err, "Add CosSim srv: %v", err)
				db.DPrintf(db.TEST, "Done scale up cossim srvs to: %v", (i+1)+ji.cfg.JobCfg.InitNSrv)
			}
		}()
	}

	for i, lg := range ji.lgs {
		db.DPrintf(db.TEST, "Run load generator rps %v dur %v", ji.cfg.MaxRPS[i], ji.cfg.Durs[i])
		lg.Run()
		//    ji.printStats()
	}
	db.DPrintf(db.ALWAYS, "Done running CosSimJob")
}

func (ji *CosSimJobInstance) printStats() {
}

func (ji *CosSimJobInstance) Wait() {
	db.DPrintf(db.TEST, "extra sleep")
	time.Sleep(20 * time.Second)
	if ji.p != nil {
		ji.p.Done()
	}
	db.DPrintf(db.TEST, "Stopping cossim job")
	if ji.sigmaos && !ji.justCli {
		ji.printStats()
		err := ji.j.Stop()
		assert.Nil(ji.Ts.T, err, "stop %v", err)
	}
	db.DPrintf(db.TEST, "Done stopping cossim job")
	for _, lg := range ji.lgs {
		db.DPrintf(db.ALWAYS, "Data:\n%v", lg.StatsDataString())
	}
	for _, lg := range ji.lgs {
		lg.Stats()
	}
}
