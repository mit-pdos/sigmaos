package benchmarks_test

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/benchmarks/loadgen"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/test"
	"sigmaos/util/perf"
)

type cosSimFn func(j *cossimsrv.CosSimJob, r *rand.Rand)

type CosSimJobInstance struct {
	sigmaos             bool
	justCli             bool
	job                 string
	dur                 []time.Duration
	maxrps              []int
	fn                  cosSimFn
	ncache              int
	scaleCacheDelay     time.Duration
	manuallyScaleCaches bool
	nCachesToAdd        int
	scaleCosSimDelay    time.Duration
	manuallyScaleCosSim bool
	nCosSimToAdd        int
	nCosSim             int
	mcpuPerSrv          proc.Tmcpu
	cosSimNVec          int
	cosSimVecDim        int
	eagerInit           bool
	ready               chan bool
	j                   *cossimsrv.CosSimJob
	lgs                 []*loadgen.LoadGenerator
	p                   *perf.Perf
	*test.RealmTstate
}

func NewCosSimJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, durs string, maxrpss string, fn cosSimFn, justCli bool, ncache int, cacheGC bool, cacheMcpu proc.Tmcpu, manuallyScaleCaches bool, scaleCacheDelay time.Duration, nCachesToAdd int, nCosSim int, cosSimNVec int, cosSimVecDim int, eagerInit bool, mcpuPerSrv proc.Tmcpu, manuallyScaleCosSim bool, scaleCosSimDelay time.Duration, nCosSimToAdd int) *CosSimJobInstance {
	ji := &CosSimJobInstance{}
	ji.job = "cossim-job"
	ji.ready = make(chan bool)
	ji.fn = fn
	ji.RealmTstate = ts
	ji.p = p
	ji.justCli = justCli
	ji.ncache = ncache
	ji.manuallyScaleCaches = manuallyScaleCaches
	ji.scaleCacheDelay = scaleCacheDelay
	ji.nCachesToAdd = nCachesToAdd
	ji.manuallyScaleCosSim = manuallyScaleCosSim
	ji.scaleCosSimDelay = scaleCosSimDelay
	ji.nCosSimToAdd = nCosSimToAdd
	ji.nCosSim = nCosSim
	ji.cosSimNVec = cosSimNVec
	ji.cosSimVecDim = cosSimVecDim
	ji.eagerInit = eagerInit
	ji.mcpuPerSrv = mcpuPerSrv

	durslice := strings.Split(durs, ",")
	maxrpsslice := strings.Split(maxrpss, ",")
	assert.Equal(ts.Ts.T, len(durslice), len(maxrpsslice), "Non-matching lengths: %v %v", durs, maxrpss)

	ji.dur = make([]time.Duration, 0, len(durslice))
	ji.maxrps = make([]int, 0, len(durslice))

	for i := range durslice {
		d, err := time.ParseDuration(durslice[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		n, err := strconv.Atoi(maxrpsslice[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		ji.dur = append(ji.dur, d)
		ji.maxrps = append(ji.maxrps, n)
	}

	var err error

	if !ji.justCli {
		// Only start one cache if autoscaling.
		ji.j, err = cossimsrv.NewCosSimJob(ts.SigmaClnt, ji.job, ji.cosSimNVec, ji.cosSimVecDim, ji.eagerInit, ji.mcpuPerSrv, ji.ncache, cacheMcpu, cacheGC)
		assert.Nil(ts.Ts.T, err, "Error NewCosSimJob: %v", err)
		for i := 0; i < ji.nCosSim; i++ {
			_, _, err := ji.j.AddSrv()
			if !assert.Nil(ts.Ts.T, err, "Err AddSrv: %v", err) {
				return ji
			}
		}
		time.Sleep(2 * time.Second)
	}

	// Make a load generators.
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.dur))
	for i := range ji.dur {
		ji.lgs = append(ji.lgs, loadgen.NewLoadGenerator(ji.dur[i], ji.maxrps[i], func(r *rand.Rand) (time.Duration, bool) {
			// Run a single request.
			ji.fn(ji.j, r)
			return 0, false
		}))
	}
	return ji
}

func (ji *CosSimJobInstance) StartCosSimJob() {
	db.DPrintf(db.ALWAYS, "StartCosSimJob dur %v ncache %v maxrps %v manuallyScaleCaches %v scaleCacheDelay %v nCachesToAdd %v manuallyScaleCosSim %v scaleCosSimDelay %v nCosSimToAdd %v nCosSimInit %v nvec %v vecdim: %v eager: %v", ji.dur, ji.ncache, ji.maxrps, ji.manuallyScaleCaches, ji.scaleCacheDelay, ji.nCachesToAdd, ji.manuallyScaleCosSim, ji.scaleCosSimDelay, ji.nCosSimToAdd, ji.nCosSim, ji.cosSimNVec, ji.cosSimVecDim, ji.eagerInit)
	var wg sync.WaitGroup
	for _, lg := range ji.lgs {
		wg.Add(1)
		go func(lg *loadgen.LoadGenerator, wg *sync.WaitGroup) {
			defer wg.Done()
			lg.Calibrate()
		}(lg, &wg)
	}
	wg.Wait()
	if !ji.justCli && ji.manuallyScaleCosSim {
		go func() {
			time.Sleep(ji.scaleCosSimDelay)
			for i := 0; i < ji.nCosSimToAdd; i++ {
				_, _, err := ji.j.AddSrv()
				assert.Nil(ji.Ts.T, err, "Add CosSim srv: %v", err)
			}
		}()
	}

	for i, lg := range ji.lgs {
		db.DPrintf(db.TEST, "Run load generator rps %v dur %v", ji.maxrps[i], ji.dur[i])
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
