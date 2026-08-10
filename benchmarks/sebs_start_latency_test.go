package benchmarks_test

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/sebs"
	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sched/msched/proc/chunk"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type SebsStartLatencyJobInstance struct {
	*test.RealmTstate
	cfg         *benchmarks.SebsBenchConfig
	ready       chan bool
	msc         *mschedclnt.MSchedClnt
	sleeperProc *proc.Proc
	warmSrvKID  string
	plainJob    *sebs.SebsJob
	wasmJob     *sebs.SebsWASMJob
}

func NewSebsStartLatencyJob(ts *test.RealmTstate, cfg *benchmarks.SebsBenchConfig) *SebsStartLatencyJobInstance {
	ji := &SebsStartLatencyJobInstance{
		RealmTstate: ts,
		cfg:         cfg,
		ready:       make(chan bool),
	}
	ji.msc = mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)

	// Spawn a sleeper proc to occupy a full node, so the warm server can cache
	// binaries without actually running the benchmark proc.
	ji.sleeperProc = proc.NewProc("sleeper", []string{fmt.Sprintf("%v", 1000*time.Second), "name/"})
	ji.sleeperProc.SetMcpu(4000)
	err := ji.Spawn(ji.sleeperProc)
	if !assert.Nil(ts.Ts.T, err, "Err Spawn sleeper: %v", err) {
		return ji
	}
	err = ji.WaitStart(ji.sleeperProc.GetPid())
	if !assert.Nil(ts.Ts.T, err, "Err WaitStart sleeper: %v", err) {
		return ji
	}

	// Discover which kernel the sleeper landed on.
	runningProcs, err := ji.msc.GetAllRunningProcs()
	if !assert.Nil(ts.Ts.T, err, "Err GetAllRunningProcs: %v", err) {
		return ji
	}
	foundSleeper := false
	for _, p := range runningProcs[ts.GetRealm()] {
		if p.GetProgram() == "sleeper" {
			ji.warmSrvKID = p.GetKernelID()
			db.DPrintf(db.TEST, "sleeper[%v] running on kernel %v", p.GetPid(), p.GetKernelID())
			foundSleeper = true
		}
	}
	if !assert.True(ts.Ts.T, foundSleeper, "Err didn't find sleeper for warm srv") {
		return ji
	}

	// Prewarm the warm server with the SeBS proc binaries.
	bundleSuffix := "tar.gz"
	if cfg.Uncompressed {
		bundleSuffix = "tar"
	}
	bins := []string{
		"sebs-runner.py-v" + sp.Version,
		cfg.Benchmark + "-bundle." + bundleSuffix,
	}
	for _, bin := range bins {
		db.DPrintf(db.TEST, "Prewarming kernel %v with bin %v", ji.warmSrvKID, bin)
		err = ji.msc.WarmProcd(ji.warmSrvKID, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), bin, ts.Ts.ProcEnv().GetSigmaPath(), proc.T_LC, "")
		if !assert.Nil(ts.Ts.T, err, "Err prewarming with bin %v: %v", bin, err) {
			return ji
		}
	}

	// Build the job. For the cosandbox path, NewSebsWASMJob fetches and
	// precompiles the WASM from S3 here — outside the measured path.
	if cfg.UseCoSandbox {
		var wasmCfg *sebs.SebsWASMJobConfig
		var factoryErr error
		switch cfg.Benchmark {
		case "210.thumbnailer":
			wasmCfg, factoryErr = sebs.NewSebsThumbnailerWASMJobConfig(cfg.Event, cfg.Kid, cfg.Mcpu, cfg.ShmemMB)
		case "220.video-processing":
			wasmCfg, factoryErr = sebs.NewSebsVideoProcessingWASMJobConfig(cfg.Event, cfg.Kid, cfg.Mcpu, cfg.ShmemMB)
		case "411.image-recognition":
			wasmCfg, factoryErr = sebs.NewSebsImageRecognitionWASMJobConfig(cfg.Event, cfg.Kid, cfg.Mcpu, cfg.ShmemMB)
		case "504.dna-visualisation":
			wasmCfg, factoryErr = sebs.NewSebsDnaVisualisationWASMJobConfig(cfg.Event, cfg.Kid, cfg.Mcpu, cfg.ShmemMB)
		default:
			assert.Fail(ts.Ts.T, "Unknown SeBS benchmark: %v", cfg.Benchmark)
			return ji
		}
		if !assert.Nil(ts.Ts.T, factoryErr, "Err building SebsWASMJobConfig for %v: %v", cfg.Benchmark, factoryErr) {
			return ji
		}
		ji.wasmJob, err = sebs.NewSebsWASMJob(wasmCfg, ts.SigmaClnt)
		if !assert.Nil(ts.Ts.T, err, "Err NewSebsWASMJob: %v", err) {
			return ji
		}
	} else {
		plainCfg := sebs.NewSebsJobConfig(cfg.Benchmark, cfg.Event, cfg.AsyncFetch, false, cfg.Uncompressed, cfg.ShmemMB, cfg.Mcpu)
		ji.plainJob, err = sebs.NewSebsJob(plainCfg, ts.SigmaClnt)
		if !assert.Nil(ts.Ts.T, err, "Err NewSebsJob: %v", err) {
			return ji
		}
	}
	return ji
}

func (ji *SebsStartLatencyJobInstance) RunJob(rs *benchmarks.Results, crash bool) bool {
	var msg string
	var err error
	if ji.cfg.UseCoSandbox {
		msg, err = ji.wasmJob.Run(chunk.ChunkdPath(ji.warmSrvKID))
	} else {
		msg, err = ji.plainJob.Run(chunk.ChunkdPath(ji.warmSrvKID))
	}
	if !assert.Nil(ji.Ts.T, err, "Err run SeBS %v (cosandbox=%v): %v", ji.cfg.Benchmark, ji.cfg.UseCoSandbox, err) {
		return false
	}
	assert.NotEmpty(ji.Ts.T, msg, "SeBS %v returned empty result", ji.cfg.Benchmark)
	db.DPrintf(db.BENCH, "SeBS %v (cosandbox=%v) result: %v", ji.cfg.Benchmark, ji.cfg.UseCoSandbox, msg)
	return true
}

func (ji *SebsStartLatencyJobInstance) Cleanup() {
	err := ji.Evict(ji.sleeperProc.GetPid())
	assert.Nil(ji.Ts.T, err, "Evict sleeper: %v", err)
	status, err := ji.WaitExit(ji.sleeperProc.GetPid())
	assert.Nil(ji.Ts.T, err, "WaitExit sleeper: %v", err)
	assert.True(ji.Ts.T, status != nil && status.IsStatusEvicted(), "Exit status wrong: %v", status)
}
