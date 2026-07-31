package remote

import (
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/apps/etcd"
	"sigmaos/apps/hotel"
	imgrec_py "sigmaos/apps/imgrec/py"
	imgrec_wasm "sigmaos/apps/imgrec/wasm"
	"sigmaos/apps/imgresize"
	"sigmaos/apps/memcached"
	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func init() {
	flag.StringVar(&platformArg, "platform", sp.NOT_SET, "Platform on which to run. Currently, only [aws|cloudlab] are supported")
	flag.StringVar(&vpcArg, "vpc", sp.NOT_SET, "VPC in which to run. Need not be specified for Cloudlab.")
	flag.StringVar(&tagArg, "build-tag", sp.NOT_SET, "Build tag with which to run.")
	flag.StringVar(&branchArg, "branch", "master", "Branch on which to run.")
	flag.StringVar(&versionArg, "bench-version", sp.NOT_SET, "Output version string.")
	flag.BoolVar(&noNetproxyArg, "no-dialproxy", false, "Disable use of proxy for network dialing/listening.")
	flag.BoolVar(&overlaysArg, "overlays", false, "Run with Docker swarm overlays enabled.")
	flag.BoolVar(&parallelArg, "parallelize", false, "Run commands in parallel to speed up, e.g., cluster shutdown.")
	flag.BoolVar(&oneByOne, "one-by-one", false, "Run one benchmark part, and then return")
	flag.BoolVar(&noShutdownArg, "no-shutdown-after-test", false, "Avoid shutting down the cluster after running a benchmark (useful for debugging).")
	flag.BoolVar(&reloadGVisor, "reload-gvisor", false, "Refresh gvisor container.")
	flag.BoolVar(&k8sArg, "k8s", false, "Run the k8s version of the experiment.")
	flag.IntVar(&mrMemReqArg, "mr_mem_req", 3000, "MR benchmarks: mem request (MB) per mapper/reducer, which is what bounds how many run concurrently per node.")
	flag.IntVar(&mrGOMAXPROCSArg, "mr_gomaxprocs", 0, "MR benchmarks: GOMAXPROCS for mapper procs (0 = Go default, i.e. the node's core count).")
	proc.SetSigmaDebugPid("remote-bench")
}

func TestCompile(t *testing.T) {
}

// Dummy test to make sure benchmark infrastructure works.
func TestInitFS(t *testing.T) {
	var (
		benchName string = "initfs"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 0
		numNodes          int  = 10
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	ts.RunStandardBenchmark(benchName, driverVM, GetInitFSCmd, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

// Example remote benchmark runner stub
func TestExample(t *testing.T) {
	var (
		benchName    string = "example"
		exampleFlag  string = "example-bench-flag-val"
		prewarmRealm bool   = false
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 0
		numNodes          int  = 10
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	getExampleCmd := GetExampleCmdConstructor(prewarmRealm, exampleFlag)
	ts.RunStandardBenchmark(benchName, driverVM, getExampleCmd, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

// Test SigmaOS cold-start.
func TestColdStart(t *testing.T) {
	var (
		benchName string = "cold_start"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 0
		numNodes          int  = 8
		numCoresPerNode   uint = 16
		numFullNodes      int  = 1
		numProcqOnlyNodes int  = 0
		turboBoost        bool = true
		useGVisor         bool = false
	)
	// Benchmark configuration parameters
	var (
		dummyProc    bool          = false
		lcProc       bool          = false
		prewarmRealm bool          = false
		skipStats    bool          = true
		rps          int           = 7
		dur          time.Duration = 5 * time.Second
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(rps, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

// Test the single-node proc start bottleneck.
func TestSingleMachineMaxTpt(t *testing.T) {
	var (
		benchNameBase string = "single_machine_max_start_tpt"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 0
		numNodes          int  = 1
		numProcqOnlyNodes int  = 0
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = true
		useGVisor         bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	// Benchmark configuration parameters
	var (
		dummyProc     bool          = false
		lcProc        bool          = false
		prewarmRealm  bool          = true
		skipStats     bool          = true
		rps           []int         = []int{400}
		nCoresPerNode []uint        = []uint{2}
		dur           time.Duration = 5 * time.Second
	)
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, nCores := range nCoresPerNode {
		for _, r := range rps {
			benchName := filepath.Join(benchNameBase, fmt.Sprintf("%v-cores-rps-%v", nCores, r))
			ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, nCores, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
		}
	}
}

// Test the maximum throughput of a single lcsched.
func TestSchedLCSchedMaxTpt(t *testing.T) {
	var (
		benchNameBase string = "lcsched_max_tpt"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 25
		numNodes          int  = 24
		numCoresPerNode   uint = 40
		numProcqOnlyNodes int  = 0
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = true
		useGVisor         bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	// Benchmark configuration parameters
	var (
		dummyProc    bool          = true
		lcProc       bool          = true
		prewarmRealm bool          = true
		skipStats    bool          = true
		rps          []int         = []int{41400, 46000, 51500, 59100}
		dur          time.Duration = 20 * time.Second
	)
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, r := range rps {
		benchName := filepath.Join(benchNameBase, fmt.Sprintf("%v-vm-rps-%v", numNodes, r))
		ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
	}
}

// Test the maximum throughput of a single procq.
func TestProcqSchedMaxTpt(t *testing.T) {
	var (
		benchNameBase string = "procq_max_tpt"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 25
		numNodes          int  = 25
		numCoresPerNode   uint = 40
		numProcqOnlyNodes int  = 1
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = true
		useGVisor         bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	// Benchmark configuration parameters
	var (
		dummyProc    bool          = true
		lcProc       bool          = false
		prewarmRealm bool          = true
		skipStats    bool          = true
		rps          []int         = []int{46000, 51500, 59100}
		dur          time.Duration = 20 * time.Second
	)
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, r := range rps {
		benchName := filepath.Join(benchNameBase, fmt.Sprintf("%v-vm-rps-%v", numNodes, r))
		ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
	}
}

// Test SigmaOS scheduling scalability (and warm-start).
func TestSchedProcStartMaxTpt(t *testing.T) {
	var (
		benchNameBase string = "proc_start_max_tpt"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 25
		numNodes          int  = 25
		numCoresPerNode   uint = 40
		numProcqOnlyNodes int  = 1
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = true
		useGVisor         bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	// Benchmark configuration parameters
	var (
		dummyProc    bool          = false
		lcProc       bool          = false
		prewarmRealm bool          = true
		skipStats    bool          = true
		rps          []int         = []int{32200, 36800, 38000, 40000, 41400}
		dur          time.Duration = 5 * time.Second
	)
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, r := range rps {
		benchName := filepath.Join(benchNameBase, fmt.Sprintf("%v-vm-rps-%v", numNodes, r))
		ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
	}
}

// Run the SigmaOS MapReduce benchmark
func TestMR(t *testing.T) {
	var (
		benchNameBase string = "mr_vs_corral"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 12
		numProcqOnlyNodes int  = 1
		turboBoost        bool = true
		useGVisor         bool = false
	)
	type MRExperimentConfig struct {
		benchName       string
		numNodes        int
		numCoresPerNode uint
		memReq          proc.Tmem
	}
	// How a mapper moves its data. Cosandboxes prefetch through the get/put
	// API, so the three entries below are the valid combinations of the two
	// knobs; the suffix names the run's results directory. The reducer has its
	// own pair of knobs, deliberately left off here so that a difference between
	// these runs is attributable to the mapper alone.
	type MRDataPathExperiment struct {
		nameSuffix     string
		useGetPut      bool
		useCosandboxes bool
	}
	// Variable MR benchmark configuration parameters
	var (
		mrApps []*MRExperimentConfig = []*MRExperimentConfig{
			{"mr-grep-wiki2G-bench-s3.json", 10, 4, 7000},
			{"mr-grep-wiki2G-granular-bench-s3.json", 54, 4, 7000},
			{"mr-wc-wiki2G-bench.json", 10, 4, 7000},
			{"mr-wc-wiki2G-bench-s3.json", 10, 4, 7000},
		}
		mrDataPaths []MRDataPathExperiment = []MRDataPathExperiment{
			// fslib streaming reader/writer
			{"", false, false},
			// UX/S3 proxy get/put RPCs
			{"-getput", true, false},
			// get/put, with a cosandbox prefetching each mapper's splits
			{"-cosandbox", true, true},
		}
		perfs         []bool = []bool{false}
		prewarmRealms []bool = []bool{true}
		//		prewarmRealms []bool   = []bool{true, false}
	)
	// Constant MR benchmark configuration parameters
	const (
		measureTpt bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, perf := range perfs {
		for _, mrEP := range mrApps {
			for _, prewarmRealm := range prewarmRealms {
				for _, dp := range mrDataPaths {
					benchName := filepath.Join(benchNameBase, mrEP.benchName)
					if prewarmRealm {
						benchName += "-warm"
					} else {
						benchName += "-cold"
					}
					if perf {
						benchName += "-perf"
					}
					benchName += dp.nameSuffix
					data := benchmarks.MRDataPathCfg{
						MapGetPut:      dp.useGetPut,
						MapCosandboxes: dp.useCosandboxes,
					}
					mrCfg, err := benchmarks.NewMRBenchConfig(mrJobDescriptionsDir, mrEP.benchName, mrEP.memReq, data)
					if !assert.Nil(ts.t, err, "Reading MR job config: %v", err) {
						return
					}
					// Stage the job's dataset on every node's host up front if it
					// reads pre-staged input; see the same call in
					// TestBEMRMultiplexing.
					if ds, ok := mrCfg.JobCfg.InputDataset(); ok {
						ts.SetInputData([]string{ds})
					}
					db.DPrintf(db.ALWAYS, "MR config: benchName %v memReq %v data %v", benchName, mrEP.memReq, data)
					numFullNodes := mrEP.numNodes - numProcqOnlyNodes
					ts.RunStandardBenchmark(benchName, driverVM, GetMRCmdConstructor(mrCfg, prewarmRealm, measureTpt, perf), mrEP.numNodes, mrEP.numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
				}
			}
		}
	}
}

func TestCorral(t *testing.T) {
	var (
		benchNameBase string = "mr_vs_corral"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 0
		numNodes          int  = 8
		numCoresPerNode   uint = 2
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = true
		useGVisor         bool = false
	)
	// Corral run configuration, app-independent part.
	const (
		corralApp    string = CorralWordCount
		corralBranch string = "play-perf-asynch"
		corralBucket string = "9ps3"
		corralInput  string = "wiki-2G/"
		corralOutput string = "output"
		corralLambda bool   = true
	)
	// Task-granularity tuning, one block per corral example app: these are the
	// values each app compiles in as its own defaults
	// (corral.SetTuningDefaults in corral/examples/<app>), restated here so
	// that a run's configuration is visible in one place and a sweep is a
	// matter of editing a number. The two apps are tuned an order of magnitude
	// apart on the map bin size and concurrency, so they get separate blocks
	// rather than one shared set that would silently be wrong for one of them.
	//
	// word_count: 130M map bins are 8 mappers with 1G of input, 16 with 2G;
	// 800M reduce bins are 16 reducers on 10G.
	const (
		wcSplitSize      int64 = 10 * 1024 * 1024
		wcMapBinSize     int64 = 130 * 1024 * 1024
		wcReduceBinSize  int64 = 160 * 1024 * 1024 * 5
		wcMaxConcurrency int   = 32
		wcMaxLineLength  int   = 2 * 1024 * 1024
	)
	// grep: 13M map bins, i.e. far more and smaller mappers than word_count,
	// and a reduce bin large enough that grep's small output lands in one
	// reducer.
	const (
		grepSplitSize      int64 = 10 * 1024 * 1024
		grepMapBinSize     int64 = 13 * 1024 * 1024
		grepReduceBinSize  int64 = 160 * 1024 * 1024 * 100
		grepMaxConcurrency int   = 200
		grepMaxLineLength  int   = 2 * 1024 * 1024
	)
	var (
		corralSplitSize      int64
		corralMapBinSize     int64
		corralReduceBinSize  int64
		corralMaxConcurrency int
		corralMaxLineLength  int
	)
	switch corralApp {
	case CorralWordCount:
		corralSplitSize = wcSplitSize
		corralMapBinSize = wcMapBinSize
		corralReduceBinSize = wcReduceBinSize
		corralMaxConcurrency = wcMaxConcurrency
		corralMaxLineLength = wcMaxLineLength
	case CorralGrep:
		corralSplitSize = grepSplitSize
		corralMapBinSize = grepMapBinSize
		corralReduceBinSize = grepReduceBinSize
		corralMaxConcurrency = grepMaxConcurrency
		corralMaxLineLength = grepMaxLineLength
	default:
		assert.Fail(t, "No tuning block for corral app %v", corralApp)
		return
	}
	// One entry per run, named for its results directory. The two 2G runs are
	// the same configuration twice: the first pays to deploy the Lambda, the
	// second finds it warm.
	corralExps := []string{"corral-2G-cold", "corral-2G-warm"}
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, exp := range corralExps {
		cfg, err := NewCorralConfig(
			corralApp,
			corralBranch,
			corralBucket,
			corralInput,
			corralOutput,
			corralLambda,
			corralSplitSize,
			corralMapBinSize,
			corralReduceBinSize,
			corralMaxConcurrency,
			corralMaxLineLength,
		)
		if !assert.Nil(ts.t, err, "Corral config: %v", err) {
			return
		}
		benchName := filepath.Join(benchNameBase, exp)
		db.DPrintf(db.ALWAYS, "Corral config: benchName %v cfg %v", benchName, cfg)
		ts.RunStandardBenchmark(benchName, driverVM, GetCorralCmdConstructor(cfg), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
	}
}

// Test Hotel application's tail latency.
func TestHotelTailLatency(t *testing.T) {
	var (
		benchName string = "hotel_tail_latency"
		driverVMs []int  = []int{8, 9, 10, 11}
	)
	// Cluster configuration parameters
	var (
		numNodes     int = 8
		numFullNodes int = numNodes
	)
	const (
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rps                 []int           = []int{250, 500, 1000, 1500, 2000, 2500}
		rpsK8s              []int           = []int{250, 500, 1000, 1500, 1500, 1500} // K8s can't support as high max load
		dur                 []time.Duration = []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second}
		numCaches           int             = 3
		cacheType           string          = "cached"
		autoscaleCache      bool            = false
		clientDelay         time.Duration   = 0 * time.Second
		sleep               time.Duration   = 0 * time.Second
		manuallyScaleCaches bool            = false
		scaleCacheDelay     time.Duration   = 0 * time.Second
		numCachesToAdd      int             = 0
		numGeo              int             = 1
		numGeoIdx           int             = 1000
		geoSearchRadius     int             = 10
		geoNResults         int             = 5
		manuallyScaleGeo    bool            = false
		scaleGeoDelay       time.Duration   = 0 * time.Second
		numGeoToAdd         int             = 0
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if ts.BCfg.Overlays {
		benchName += "_overlays"
	}
	if ts.BCfg.K8s {
		benchName += "_k8s"
		rps = rpsK8s
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	hotelCfg := &benchmarks.HotelBenchConfig{
		JobCfg: &hotel.HotelJobConfig{
			Job:             "hotel-job",
			Srvs:            hotel.NewHotelSvc(),
			NHotel:          80,
			Cache:           cacheType,
			CacheCfg:        nil,
			ImgSizeMB:       0,
			NGeo:            numGeo,
			NGeoIdx:         numGeoIdx,
			GeoSearchRadius: geoSearchRadius,
			GeoNResults:     geoNResults,
			UseMatch:        false,
		},
		Durs:           dur,
		MaxRPS:         rps,
		CachedUserFrac: 100,
		ScaleGeo: &benchmarks.ManualScalingConfig{
			Svc:         "hotel-geo",
			Scale:       manuallyScaleGeo,
			ScaleDelays: []time.Duration{scaleGeoDelay},
			ScaleDeltas: []int{numGeoToAdd},
		},
		CacheBenchCfg: &benchmarks.CacheBenchConfig{
			JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
			Shmem:     true,
			Autoscale: autoscaleCache,
			ManuallyScale: &benchmarks.ManualScalingConfig{
				Svc:         "cached",
				Scale:       manuallyScaleCaches,
				ScaleDelays: []time.Duration{scaleCacheDelay},
				ScaleDeltas: []int{numCachesToAdd},
			},
			Migrate: &benchmarks.MigrationConfig{
				Svc:              "cached",
				Migrate:          false,
				MigrationDelays:  []time.Duration{},
				MigrationTargets: []int{},
			},
		},
		CosSimBenchCfg: nil,
	}
	getLeaderCmd := GetHotelClientCmdConstructor("Search", true, len(driverVMs), sleep, hotelCfg)
	getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), sleep, hotelCfg)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

// Test Hotel application's tail latency.
func TestHotelScaleGeo(t *testing.T) {
	var (
		benchNameBase string = "hotel_tail_latency"
		driverVMs     []int  = []int{9, 10, 11, 12}
		driverVMsK8s  []int  = []int{8, 9, 10, 11}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 9
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rps                []int           = []int{250, 750, 1500}
		dur                []time.Duration = []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second}
		numGeoBase         int             = 1
		numCaches          int             = 3
		cacheType          string          = "cached"
		autoscaleCache     bool            = false
		clientDelay        time.Duration   = 0 * time.Second
		sleep              time.Duration   = 0 * time.Second
		numGeoIdx          int             = 1
		geoSearchRadius    int             = 10
		geoNResults        int             = 5
		manuallyScaleGeo   []bool          = []bool{true, false}
		scaleGeoDelayBase  time.Duration   = 20 * time.Second
		scaleGeoExtraDelay []time.Duration = []time.Duration{0, 1 * time.Second}
		nAdditionalGeo     []int           = []int{0, 2}
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if ts.BCfg.Overlays {
		benchNameBase += "_overlays"
	}
	if ts.BCfg.K8s {
		benchNameBase += "_k8s"
		driverVMs = driverVMsK8s
	}
	for _, scale := range manuallyScaleGeo {
		for _, numGeoToAdd := range nAdditionalGeo {
			for _, extraDelay := range scaleGeoExtraDelay {
				// Don't add artificial delays for k8s
				if ts.BCfg.K8s {
					extraDelay = 0
				}
				db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
				benchName := benchNameBase
				numGeo := numGeoBase
				scaleGeoDelay := scaleGeoDelayBase
				if scale {
					benchName += "_scale_geo_add_" + strconv.Itoa(numGeoToAdd)
					if extraDelay > 0 && numGeoToAdd > 0 {
						scaleGeoDelay += extraDelay
						benchName += "_extra_scaling_delay_" + extraDelay.String()
					}
				} else {
					numGeo += numGeoToAdd
					benchName += "_no_scale_geo_ngeo_" + strconv.Itoa(numGeo)
				}
				hotelCfg := &benchmarks.HotelBenchConfig{
					JobCfg: &hotel.HotelJobConfig{
						Job:             "hotel-job",
						Srvs:            hotel.NewHotelSvc(),
						NHotel:          80,
						Cache:           cacheType,
						CacheCfg:        nil,
						ImgSizeMB:       0,
						NGeo:            numGeo,
						NGeoIdx:         numGeoIdx,
						GeoSearchRadius: geoSearchRadius,
						GeoNResults:     geoNResults,
						UseMatch:        false,
					},
					Durs:           dur,
					MaxRPS:         rps,
					CachedUserFrac: 100,
					ScaleGeo: &benchmarks.ManualScalingConfig{
						Svc:         "hotel-geo",
						Scale:       scale,
						ScaleDelays: []time.Duration{scaleGeoDelay},
						ScaleDeltas: []int{numGeoToAdd},
					},
					CacheBenchCfg: &benchmarks.CacheBenchConfig{
						JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
						Shmem:     true,
						Autoscale: autoscaleCache,
						ManuallyScale: &benchmarks.ManualScalingConfig{
							Svc:         "cached",
							Scale:       false,
							ScaleDelays: []time.Duration{},
							ScaleDeltas: []int{},
						},
					},
					CosSimBenchCfg: nil,
				}
				getLeaderCmd := GetHotelClientCmdConstructor("Search", true, len(driverVMs), sleep, hotelCfg)
				getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), sleep, hotelCfg)
				ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
			}
		}
	}
}

// Test Hotel Geo's application tail latency.
func TestHotelGeoReqScaleGeo(t *testing.T) {
	var (
		benchNameBase string = "hotel_tail_latency_georeq"
		driverVMs     []int  = []int{9}
		driverVMsK8s  []int  = []int{9}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 9
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rps                []int           = []int{250, 500, 750}
		dur                []time.Duration = []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second}
		numGeoBase         int             = 1
		numCaches          int             = 3
		cacheType          string          = "cached"
		autoscaleCache     bool            = false
		clientDelay        time.Duration   = 0 * time.Second
		sleep              time.Duration   = 0 * time.Second
		geoSearchRadius    int             = 20
		geoNResults        int             = 500
		numGeoIdx          int             = 1
		manuallyScaleGeo   []bool          = []bool{true, false}
		scaleGeoDelayBase  time.Duration   = 20 * time.Second
		scaleGeoExtraDelay []time.Duration = []time.Duration{0}
		nAdditionalGeo     []int           = []int{2, 0}
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if ts.BCfg.Overlays {
		benchNameBase += "_overlays"
	}
	if ts.BCfg.K8s {
		benchNameBase += "_k8s"
		driverVMs = driverVMsK8s
	}
	for _, scale := range manuallyScaleGeo {
		for _, numGeoToAdd := range nAdditionalGeo {
			for _, extraDelay := range scaleGeoExtraDelay {
				// Don't add artificial delays for k8s
				if ts.BCfg.K8s {
					extraDelay = 0
				}
				db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
				benchName := benchNameBase
				numGeo := numGeoBase
				scaleGeoDelay := scaleGeoDelayBase
				if scale {
					if numGeoToAdd == 0 {
						continue
					}
					benchName += "_scale_geo_add_" + strconv.Itoa(numGeoToAdd)
					if extraDelay > 0 && numGeoToAdd > 0 {
						scaleGeoDelay += extraDelay
						benchName += "_extra_scaling_delay_" + extraDelay.String()
					}
				} else {
					numGeo += numGeoToAdd
					benchName += "_no_scale_geo_ngeo_" + strconv.Itoa(numGeo)
				}
				hotelCfg := &benchmarks.HotelBenchConfig{
					JobCfg: &hotel.HotelJobConfig{
						Job:             "hotel-job",
						Srvs:            hotel.NewHotelSvc(),
						NHotel:          80,
						Cache:           cacheType,
						CacheCfg:        &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
						ImgSizeMB:       0,
						NGeo:            numGeo,
						NGeoIdx:         numGeoIdx,
						GeoSearchRadius: geoSearchRadius,
						GeoNResults:     geoNResults,
						UseMatch:        false,
					},
					Durs:           dur,
					MaxRPS:         rps,
					CachedUserFrac: 100,
					ScaleGeo: &benchmarks.ManualScalingConfig{
						Svc:         "hotel-geo",
						Scale:       scale,
						ScaleDelays: []time.Duration{scaleGeoDelay},
						ScaleDeltas: []int{numGeoToAdd},
					},
					CacheBenchCfg: &benchmarks.CacheBenchConfig{
						JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
						Shmem:     true,
						Autoscale: autoscaleCache,
						ManuallyScale: &benchmarks.ManualScalingConfig{
							Svc:         "cached",
							Scale:       false,
							ScaleDelays: []time.Duration{},
							ScaleDeltas: []int{},
						},
					},
					CosSimBenchCfg: nil,
				}
				getLeaderCmd := GetHotelClientCmdConstructor("Geo", true, len(driverVMs), sleep, hotelCfg)
				getFollowerCmd := GetHotelClientCmdConstructor("Geo", false, len(driverVMs), sleep, hotelCfg)
				ran := ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
				if oneByOne && ran {
					return
				}
			}
		}
	}
}

// Test Hotel application's tail latency.
func TestHotelScaleCache(t *testing.T) {
	var (
		benchNameBase string = "hotel_tail_latency"
		driverVMs     []int  = []int{8, 9, 10, 11}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 8
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rps                  []int           = []int{250, 1500, 2500}
		dur                  []time.Duration = []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second}
		numCachesBase        int             = 1
		cacheType            string          = "cached"
		autoscaleCache       bool            = false
		clientDelay          time.Duration   = 0 * time.Second
		sleep                time.Duration   = 0 * time.Second
		manuallyScaleCaches  []bool          = []bool{true, false}
		scaleCacheDelayBase  time.Duration   = 20 * time.Second
		scaleCacheExtraDelay []time.Duration = []time.Duration{0, 200 * time.Millisecond, 500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
		nAdditionalCaches    []int           = []int{0, 2}
		geoSearchRadius      int             = 10
		geoNResults          int             = 5
		numGeoIdx            int             = 1000
		numGeo               int             = 1
		manuallyScaleGeo     bool            = false
		scaleGeoDelay        time.Duration   = 0 * time.Second
		numGeoToAdd          int             = 0
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if ts.BCfg.Overlays {
		benchNameBase += "_overlays"
	}
	if ts.BCfg.K8s {
		benchNameBase += "_k8s"
	}
	for _, scale := range manuallyScaleCaches {
		for _, numCachesToAdd := range nAdditionalCaches {
			for _, extraDelay := range scaleCacheExtraDelay {
				db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
				benchName := benchNameBase
				numCaches := numCachesBase
				scaleCacheDelay := scaleCacheDelayBase
				if scale {
					benchName += "_scale_cache_add_" + strconv.Itoa(numCachesToAdd)
					if extraDelay > 0 && numCachesToAdd > 0 {
						scaleCacheDelay += extraDelay
						benchName += "_extra_scaling_delay_" + extraDelay.String()
					}
				} else {
					numCaches += numCachesToAdd
					benchName += "_no_scale_cache_ncache_" + strconv.Itoa(numCaches)
				}
				hotelCfg := &benchmarks.HotelBenchConfig{
					JobCfg: &hotel.HotelJobConfig{
						Job:             "hotel-job",
						Srvs:            hotel.NewHotelSvc(),
						NHotel:          80,
						Cache:           cacheType,
						CacheCfg:        &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
						ImgSizeMB:       0,
						NGeo:            numGeo,
						NGeoIdx:         numGeoIdx,
						GeoSearchRadius: geoSearchRadius,
						GeoNResults:     geoNResults,
						UseMatch:        false,
					},
					Durs:           dur,
					MaxRPS:         rps,
					CachedUserFrac: 100,
					ScaleGeo: &benchmarks.ManualScalingConfig{
						Svc:         "hotel-geo",
						Scale:       manuallyScaleGeo,
						ScaleDelays: []time.Duration{scaleGeoDelay},
						ScaleDeltas: []int{numGeoToAdd},
					},
					CacheBenchCfg: &benchmarks.CacheBenchConfig{
						JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
						Shmem:     true,
						Autoscale: autoscaleCache,
						ManuallyScale: &benchmarks.ManualScalingConfig{
							Svc:         "cached",
							Scale:       scale,
							ScaleDelays: []time.Duration{scaleCacheDelay},
							ScaleDeltas: []int{numCachesToAdd},
						},
					},
					CosSimBenchCfg: nil,
				}
				getLeaderCmd := GetHotelClientCmdConstructor("Search", true, len(driverVMs), sleep, hotelCfg)
				getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), sleep, hotelCfg)
				ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
			}
		}
	}
}

// Test Socialnet application's tail latency.
func TestSocialnetTailLatency(t *testing.T) {
	var (
		benchName string = "socialnet_tail_latency"
		driverVMs []int  = []int{8}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 8
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Socialnet benchmark configuration parameters
	var (
		rps         []int           = []int{1000, 2000, 4000, 6000}
		dur         []time.Duration = []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second}
		clientDelay time.Duration   = 40 * time.Second
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if ts.BCfg.Overlays {
		benchName += "_overlays"
	}
	if ts.BCfg.K8s {
		benchName += "_k8s"
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	getLeaderCmd := GetSocialnetClientCmdConstructor(true, len(driverVMs), rps, dur)
	getFollowerCmd := GetSocialnetClientCmdConstructor(false, len(driverVMs), rps, dur)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sSocialnetApp, stopK8sSocialnetApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

// Test multiplexing Best Effort ImgResize jobs.
func TestBEImgResizeMultiplexing(t *testing.T) {
	var (
		benchName string = "be_imgresize_multiplexing"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 12
		numNodes          int  = 8 // 24
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 0
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = false
		useGVisor         bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	imgCfg := &benchmarks.ImgBenchConfig{
		JobCfg: &imgresize.ImgdJobConfig{
			Job:                 "img-job",
			WorkerMcpu:          proc.Tmcpu(0),
			WorkerMem:           proc.Tmem(1500),
			Persist:             false,
			NRounds:             300,
			ImgdMcpu:            proc.Tmcpu(1000),
			UseSPProxy:          false,
			UseCoSandbox:        false,
			UseS3Clnt:           false,
			WorkerCoSandboxMcpu: proc.Tmcpu(0),
			WorkerCoSandboxMem:  proc.Tmem(0),
			FTTaskSrvMcpu:       proc.Tmcpu(1000),
		},
		InputPath:      "name/ux/~local/8.jpg",
		NTasks:         10,
		NInputsPerTask: 25,
	}
	ts.RunStandardBenchmark(benchName, driverVM, GetBEImgResizeMultiplexingCmdConstructor(4, 5*time.Second, imgCfg), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

// Test multiplexing Best Effort ImgResize jobs.
func TestBEImgResizeRPCMultiplexing(t *testing.T) {
	var (
		benchName string = "be_imgresize_rpc_multiplexing"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 12
		numNodes          int  = 10
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 2
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Bench params
	const (
		sleepBetweenRealms time.Duration = 5 * time.Second
		nRealms            int           = 4
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	imgCfg := &benchmarks.ImgBenchConfig{
		JobCfg: &imgresize.ImgdJobConfig{
			Job:                 "img-job",
			WorkerMcpu:          proc.Tmcpu(0),
			WorkerMem:           proc.Tmem(2500),
			Persist:             false,
			NRounds:             43,
			ImgdMcpu:            proc.Tmcpu(1000),
			UseSPProxy:          false,
			UseCoSandbox:        false,
			UseS3Clnt:           false,
			WorkerCoSandboxMcpu: proc.Tmcpu(0),
			WorkerCoSandboxMem:  proc.Tmem(0),
			FTTaskSrvMcpu:       proc.Tmcpu(1000),
		},
		InputPath:      "name/ux/~local/8.jpg",
		NTasks:         20000,
		NInputsPerTask: 43,
		//		Durs:           []time.Duration{20 * time.Second},
		//		MaxRPS:         []int{500},
		// Duration-based spawning no longer supported
	}
	ts.RunStandardBenchmark(benchName, driverVM, GetBEImgResizeRPCMultiplexingCmdConstructor(nRealms, sleepBetweenRealms, imgCfg), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

// Test multiplexing Best Effort MR jobs.
func TestBEMRMultiplexing(t *testing.T) {
	const (
		benchNameBase string = "be_mr_multiplexing"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 36
		numNodes          int  = 32
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 2
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Bench params
	const (
		sleepBetweenRealms time.Duration = 10 * time.Second
		nRealms            int           = 4
		prewarmRealm       bool          = true
		useGetPut          bool          = false
		// The reducer data path is a separate pair of knobs from the mapper's;
		// the sweep below is over the mapper's, so set these explicitly rather
		// than letting the sweep imply them.
		useGetPutReduce      bool = false
		useCosandboxesReduce bool = false
		// Shared memory per cosandbox reducer, which has to hold every shard
		// the cosandbox prefetches for it: all of the mappers' output divided by
		// nreduce. 0 lets the coordinator size it from what the mappers actually
		// wrote. Only used when useCosandboxesReduce is set.
		reduceShmemMB int = 0
		//		benchConfig        string        = "mr-grep-wiki1G-granular-bench.json"
		benchConfig string = "mr-grep-uxinput-granular-bench.json"
	)
	// Run the benchmark with mappers reading and writing directly, and then
	// through cosandboxes.
	cosandboxCfgs := []bool{false} //, true}
	// Mem request per worker is what bounds concurrent mappers per node
	// (msched admits on memory), so it is the knob for the packing sweep:
	// --mr_mem_req 8000 ~1/node, 3000 ~5/node, 1500 ~10/node, 1200 ~13/node.
	memPerWorker := proc.Tmem(mrMemReqArg)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, useCosandboxes := range cosandboxCfgs {
		// Each configuration gets its own results directory, so a sweep doesn't
		// overwrite itself.
		benchName := fmt.Sprintf("%s_mem%d", benchNameBase, mrMemReqArg)
		if mrGOMAXPROCSArg > 0 {
			benchName = fmt.Sprintf("%s_gomaxprocs%d", benchName, mrGOMAXPROCSArg)
		}
		if useCosandboxes {
			benchName = benchName + "_cosandboxes"
		}
		data := benchmarks.MRDataPathCfg{
			MapGetPut:         useGetPut,
			MapCosandboxes:    useCosandboxes,
			ReduceGetPut:      useGetPutReduce,
			ReduceCosandboxes: useCosandboxesReduce,
		}
		mrCfg, err := benchmarks.NewMRBenchConfig(mrJobDescriptionsDir, benchConfig, memPerWorker, data)
		if !assert.Nil(ts.t, err, "Reading MR job config: %v", err) {
			return
		}
		mrCfg.JobCfg.MapperGOMAXPROCS = mrGOMAXPROCSArg
		mrCfg.JobCfg.ReduceShmemMB = reduceShmemMB
		// If the job reads pre-staged input from UX, have every node stage that
		// dataset on its host before its kernel starts, rather than copying it
		// from S3 through the namespace once the cluster is up. Taken from the
		// job description so the staged dataset is by construction the one the
		// job reads.
		if ds, ok := mrCfg.JobCfg.InputDataset(); ok {
			ts.SetInputData([]string{ds})
		}
		db.DPrintf(db.ALWAYS, "MR multiplexing config: benchName %v memPerWorker %v mapperGOMAXPROCS %v reduceShmemMB %v data %v", benchName, memPerWorker, mrGOMAXPROCSArg, reduceShmemMB, data)
		// Each run stops any previously running cluster and starts a fresh one.
		ts.RunStandardBenchmark(benchName, driverVM, GetBEMRMultiplexingCmdConstructor(nRealms, sleepBetweenRealms, prewarmRealm, mrCfg), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
	}
}

func TestLCBEHotelImgResizeMultiplexing(t *testing.T) {
	var (
		benchName string = "lc_be_hotel_imgresize_multiplexing"
		driverVMs []int  = []int{8, 9, 10, 11}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 8
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rps                 []int           = []int{250, 500, 1000, 1500, 2000, 1000}
		dur                 []time.Duration = []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 20 * time.Second, 15 * time.Second}
		numCaches           int             = 3
		cacheType           string          = "cached"
		autoscaleCache      bool            = false
		clientDelay         time.Duration   = 60 * time.Second
		sleep               time.Duration   = 10 * time.Second
		manuallyScaleCaches bool            = false
		scaleCacheDelay     time.Duration   = 0 * time.Second
		numCachesToAdd      int             = 0
		numGeo              int             = 1
		geoSearchRadius     int             = 10
		geoNResults         int             = 5
		numGeoIdx           int             = 1000
		manuallyScaleGeo    bool            = false
		scaleGeoDelay       time.Duration   = 0 * time.Second
		numGeoToAdd         int             = 0
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	hotelCfg := &benchmarks.HotelBenchConfig{
		JobCfg: &hotel.HotelJobConfig{
			Job:             "hotel-job",
			Srvs:            hotel.NewHotelSvc(),
			NHotel:          80,
			Cache:           cacheType,
			CacheCfg:        &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
			ImgSizeMB:       0,
			NGeo:            numGeo,
			NGeoIdx:         numGeoIdx,
			GeoSearchRadius: geoSearchRadius,
			GeoNResults:     geoNResults,
			UseMatch:        false,
		},
		Durs:           dur,
		MaxRPS:         rps,
		CachedUserFrac: 100,
		ScaleGeo: &benchmarks.ManualScalingConfig{
			Svc:         "hotel-geo",
			Scale:       manuallyScaleGeo,
			ScaleDelays: []time.Duration{scaleGeoDelay},
			ScaleDeltas: []int{numGeoToAdd},
		},
		CacheBenchCfg: &benchmarks.CacheBenchConfig{
			JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
			Shmem:     true,
			Autoscale: autoscaleCache,
			ManuallyScale: &benchmarks.ManualScalingConfig{
				Svc:         "cached",
				Scale:       manuallyScaleCaches,
				ScaleDelays: []time.Duration{scaleCacheDelay},
				ScaleDeltas: []int{numCachesToAdd},
			},
			Migrate: &benchmarks.MigrationConfig{
				Svc:              "cached",
				Migrate:          false,
				MigrationDelays:  []time.Duration{},
				MigrationTargets: []int{},
			},
		},
		CosSimBenchCfg: nil,
	}
	imgCfg := &benchmarks.ImgBenchConfig{
		JobCfg: &imgresize.ImgdJobConfig{
			Job:                 "img-job",
			WorkerMcpu:          proc.Tmcpu(0),
			WorkerMem:           proc.Tmem(1500),
			Persist:             false,
			NRounds:             500,
			ImgdMcpu:            proc.Tmcpu(1000),
			UseSPProxy:          false,
			UseCoSandbox:        false,
			UseS3Clnt:           false,
			WorkerCoSandboxMcpu: proc.Tmcpu(0),
			WorkerCoSandboxMem:  proc.Tmem(0),
			FTTaskSrvMcpu:       proc.Tmcpu(1000),
		},
		InputPath:      "name/ux/~local/8.jpg",
		NTasks:         350,
		NInputsPerTask: 1,
		Durs:           nil,
		MaxRPS:         nil,
	}
	getLeaderCmd := GetLCBEHotelImgResizeMultiplexingCmdConstructor(len(driverVMs), rps, dur, cacheType, autoscaleCache, sleep, imgCfg)
	getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), sleep, hotelCfg)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, nil, nil, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

func TestLCBEHotelImgResizeRPCMultiplexing(t *testing.T) {
	var (
		benchName string = "lc_be_hotel_imgresize_rpc_multiplexing"
		driverVMs []int  = []int{8, 9, 10, 11}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 8
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 0
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rps                 []int           = []int{250, 500, 1000, 1500, 2000, 1000}
		dur                 []time.Duration = []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 20 * time.Second, 15 * time.Second}
		numCaches           int             = 3
		cacheType           string          = "cached"
		autoscaleCache      bool            = false
		clientDelay         time.Duration   = 60 * time.Second
		sleep               time.Duration   = 10 * time.Second
		manuallyScaleCaches bool            = false
		scaleCacheDelay     time.Duration   = 0 * time.Second
		numCachesToAdd      int             = 0
		numGeo              int             = 1
		geoSearchRadius     int             = 10
		geoNResults         int             = 5
		numGeoIdx           int             = 1000
		manuallyScaleGeo    bool            = false
		scaleGeoDelay       time.Duration   = 0 * time.Second
		numGeoToAdd         int             = 0
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	hotelCfg := &benchmarks.HotelBenchConfig{
		JobCfg: &hotel.HotelJobConfig{
			Job:             "hotel-job",
			Srvs:            hotel.NewHotelSvc(),
			NHotel:          80,
			Cache:           cacheType,
			CacheCfg:        &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
			ImgSizeMB:       0,
			NGeo:            numGeo,
			NGeoIdx:         numGeoIdx,
			GeoSearchRadius: geoSearchRadius,
			GeoNResults:     geoNResults,
			UseMatch:        false,
		},
		Durs:           dur,
		MaxRPS:         rps,
		CachedUserFrac: 100,
		ScaleGeo: &benchmarks.ManualScalingConfig{
			Svc:         "hotel-geo",
			Scale:       manuallyScaleGeo,
			ScaleDelays: []time.Duration{scaleGeoDelay},
			ScaleDeltas: []int{numGeoToAdd},
		},
		CacheBenchCfg: &benchmarks.CacheBenchConfig{
			JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
			Shmem:     true,
			Autoscale: autoscaleCache,
			ManuallyScale: &benchmarks.ManualScalingConfig{
				Svc:         "cached",
				Scale:       manuallyScaleCaches,
				ScaleDelays: []time.Duration{scaleCacheDelay},
				ScaleDeltas: []int{numCachesToAdd},
			},
			Migrate: &benchmarks.MigrationConfig{
				Svc:              "cached",
				Migrate:          false,
				MigrationDelays:  []time.Duration{},
				MigrationTargets: []int{},
			},
		},
		CosSimBenchCfg: nil,
	}
	imgCfg := &benchmarks.ImgBenchConfig{
		JobCfg: &imgresize.ImgdJobConfig{
			Job:                 "img-job",
			WorkerMcpu:          proc.Tmcpu(0),
			WorkerMem:           proc.Tmem(2500),
			Persist:             false,
			NRounds:             43,
			ImgdMcpu:            proc.Tmcpu(1000),
			UseSPProxy:          false,
			UseCoSandbox:        false,
			UseS3Clnt:           false,
			WorkerCoSandboxMcpu: proc.Tmcpu(0),
			WorkerCoSandboxMem:  proc.Tmem(0),
			FTTaskSrvMcpu:       proc.Tmcpu(1000),
		},
		InputPath:      "name/ux/~local/8.jpg",
		NTasks:         0,
		NInputsPerTask: 0,
		Durs:           []time.Duration{50 * time.Second},
		MaxRPS:         []int{150},
	}
	getLeaderCmd := GetLCBEHotelImgResizeRPCMultiplexingCmdConstructor(len(driverVMs), rps, dur, cacheType, autoscaleCache, sleep, imgCfg)
	getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), sleep, hotelCfg)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, nil, nil, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

// Test multiplexing an LC hotel job with a BE MR job.
func TestLCBEHotelMRMultiplexing(t *testing.T) {
	var (
		benchName string = "lc_be_hotel_mr_multiplexing"
		driverVMs []int  = []int{8, 9, 10, 11}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 8
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 0
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rps                 []int           = []int{250, 500, 1000, 1500, 2000, 1000}
		dur                 []time.Duration = []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 20 * time.Second, 15 * time.Second}
		numCaches           int             = 3
		cacheType           string          = "cached"
		autoscaleCache      bool            = false
		clientDelay         time.Duration   = 60 * time.Second
		sleep               time.Duration   = 10 * time.Second
		manuallyScaleCaches bool            = false
		scaleCacheDelay     time.Duration   = 0 * time.Second
		numCachesToAdd      int             = 0
		numGeo              int             = 1
		geoSearchRadius     int             = 10
		geoNResults         int             = 5
		numGeoIdx           int             = 1000
		manuallyScaleGeo    bool            = false
		scaleGeoDelay       time.Duration   = 0 * time.Second
		numGeoToAdd         int             = 0
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	hotelCfg := &benchmarks.HotelBenchConfig{
		JobCfg: &hotel.HotelJobConfig{
			Job:             "hotel-job",
			Srvs:            hotel.NewHotelSvc(),
			NHotel:          80,
			Cache:           cacheType,
			CacheCfg:        &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
			ImgSizeMB:       0,
			NGeo:            numGeo,
			NGeoIdx:         numGeoIdx,
			GeoSearchRadius: geoSearchRadius,
			GeoNResults:     geoNResults,
			UseMatch:        false,
		},
		Durs:           dur,
		MaxRPS:         rps,
		CachedUserFrac: 100,
		ScaleGeo: &benchmarks.ManualScalingConfig{
			Svc:         "hotel-geo",
			Scale:       manuallyScaleGeo,
			ScaleDelays: []time.Duration{scaleGeoDelay},
			ScaleDeltas: []int{numGeoToAdd},
		},
		CacheBenchCfg: &benchmarks.CacheBenchConfig{
			JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
			Shmem:     true,
			Autoscale: autoscaleCache,
			ManuallyScale: &benchmarks.ManualScalingConfig{
				Svc:         "cached",
				Scale:       manuallyScaleCaches,
				ScaleDelays: []time.Duration{scaleCacheDelay},
				ScaleDeltas: []int{numCachesToAdd},
			},
			Migrate: &benchmarks.MigrationConfig{
				Svc:              "cached",
				Migrate:          false,
				MigrationDelays:  []time.Duration{},
				MigrationTargets: []int{},
			},
		},
		CosSimBenchCfg: nil,
	}
	mrCfg, err := benchmarks.NewMRBenchConfig(mrJobDescriptionsDir, "mr-grep-wiki2G-bench-s3.json", proc.Tmem(7000), benchmarks.MRDataPathCfg{})
	if !assert.Nil(ts.t, err, "Reading MR job config: %v", err) {
		return
	}
	getLeaderCmd := GetLCBEHotelMRMultiplexingCmdConstructor(len(driverVMs), sleep, hotelCfg, mrCfg)
	getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), sleep, hotelCfg)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, nil, nil, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
}

// Test CosSim's application tail latency.
func TestScaleCosSim(t *testing.T) {
	var (
		benchNameBase string = "cos_sim_tail_latency"
		driverVMs     []int  = []int{9}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 8
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// CosSim benchmark configuration parameters
	var (
		numCosSimBase         int             = 1
		nCache                []int           = []int{1, 2, 4}
		clientDelay           time.Duration   = 0 * time.Second
		sleep                 time.Duration   = 0 * time.Second
		useCoSandbox          []bool          = []bool{true, false}
		manuallyScaleCosSim   []bool          = []bool{true, false}
		scaleCosSimDelayBase  time.Duration   = 35 * time.Second
		scaleCosSimExtraDelay []time.Duration = []time.Duration{0}
		nAdditionalCosSim     []int           = []int{0, 1}
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	for _, numCaches := range nCache {
		for _, delegate := range useCoSandbox {
			for _, scale := range manuallyScaleCosSim {
				for _, numCosSimToAdd := range nAdditionalCosSim {
					for _, extraDelay := range scaleCosSimExtraDelay {
						// Don't add artificial delays for k8s
						if ts.BCfg.K8s {
							extraDelay = 0
						}
						db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
						benchName := benchNameBase + "_ncache_" + strconv.Itoa(numCaches)
						numCosSim := numCosSimBase
						scaleCosSimDelay := scaleCosSimDelayBase
						benchName += "_eager"
						if delegate {
							benchName += "_delegate"
						}
						if scale {
							if numCosSimToAdd == 0 {
								continue
							}
							benchName += "_scale_cossim_add_" + strconv.Itoa(numCosSimToAdd)
							if extraDelay > 0 && numCosSimToAdd > 0 {
								scaleCosSimDelay += extraDelay
								benchName += "_extra_scaling_delay_" + extraDelay.String()
							}
						} else {
							if numCosSimToAdd == 0 {
								continue
							}
							// RPC delegation not interesting without scaling happening
							if delegate {
								continue
							}
							numCosSim += numCosSimToAdd
							benchName += "_no_scale_cossim_nsrv_" + strconv.Itoa(numCosSim)
						}
						cacheCfg := cachegrpmgr.NewCacheJobConfig(numCaches, 2000, true)
						jobCfg := cossimsrv.NewCosSimJobConfig("cossim", numCosSim, 10000, 100, true, 4000, cacheCfg, delegate)
						scaleCosSim := benchmarks.NewManualScalingConfig("cossim", scale, []time.Duration{scaleCosSimDelay}, []int{numCosSimToAdd})
						cfg := &benchmarks.CosSimBenchConfig{
							JobCfg:        jobCfg,
							NVecToQuery:   5000,
							Durs:          []time.Duration{5 * time.Second, 30 * time.Second, 30 * time.Second},
							MaxRPS:        []int{300, 500, 1000},
							ManuallyScale: scaleCosSim,
							Autoscale:     &benchmarks.AutoscalingConfig{Scale: false},
						}
						getLeaderCmd := GetCosSimClientCmdConstructor("CosSim", true, len(driverVMs), sleep, cfg)
						getFollowerCmd := GetCosSimClientCmdConstructor("CosSim", false, len(driverVMs), sleep, cfg)
						ran := ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
						if oneByOne && ran {
							return
						}
					}
				}
			}
		}
	}
}

// Test Cached scaler's application tail latency.
func TestScaleCachedScaler(t *testing.T) {
	var (
		benchNameBase string = "cached_scaler_tail_latency"
		driverVMs     []int  = []int{6}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 5
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Cached benchmark configuration parameters
	var (
		clientDelay      time.Duration = 0 * time.Second
		sleep            time.Duration = 0 * time.Second
		useCoSandbox     []bool        = []bool{true, false}
		cppCached        []bool        = []bool{true, false}
		prewarmRealm     []bool        = []bool{false} //[]bool{true, false}
		useEPCache       bool          = true
		scale            bool          = true
		scaleDelay                     = 5 * time.Second
		useCossimBackend []bool        = []bool{true} //[]bool{true, false}
		cacheMcpu        proc.Tmcpu    = 4000
		cossimMcpu       proc.Tmcpu    = 4000
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	for _, prewarm := range prewarmRealm {
		for _, cpp := range cppCached {
			for _, delegate := range useCoSandbox {
				for _, cossimBackend := range useCossimBackend {
					db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
					benchName := benchNameBase
					if cpp {
						benchName += "_cpp"
					}
					if delegate {
						benchName += "_delegate"
					}
					if prewarm {
						benchName += "_prewarm"
					}
					if cossimBackend {
						benchName += "_cossim_backend"
					}
					// Create CacheBenchConfig
					cacheCfg := cachegrpmgr.NewCacheJobConfig(1, cacheMcpu, true)
					scaleCached := benchmarks.NewManualScalingConfig("cached", scale, []time.Duration{scaleDelay}, []int{1})
					cacheBenchCfg := &benchmarks.CacheBenchConfig{
						JobCfg:        cacheCfg,
						CPP:           cpp,
						RunSleeper:    true,
						Shmem:         true,
						CosSimBackend: cossimBackend,
						UseEPCache:    useEPCache,
						UseCoSandbox:  delegate,
						Autoscale:     false,
						NKeys:         5000,
						TopNShards:    1,
						Durs:          []time.Duration{30 * time.Second},
						MaxRPS:        []int{2000},
						PutDurs:       []time.Duration{0 * time.Second},
						PutMaxRPS:     []int{0},
						ManuallyScale: scaleCached,
						Migrate: &benchmarks.MigrationConfig{
							Svc:              "cached",
							Migrate:          false,
							MigrationDelays:  []time.Duration{},
							MigrationTargets: []int{},
						},
					}
					// Create CosSimBenchConfig
					var cosSimBenchCfg *benchmarks.CosSimBenchConfig
					if cossimBackend {
						cossimCacheCfg := cachegrpmgr.NewCacheJobConfig(1, cacheMcpu, true)
						cossimJobCfg := cossimsrv.NewCosSimJobConfig("cossim", 1, 10000, 100, true, cossimMcpu, cossimCacheCfg, false)
						cosSimBenchCfg = &benchmarks.CosSimBenchConfig{
							JobCfg:        cossimJobCfg,
							NVecToQuery:   5000,
							Durs:          []time.Duration{30 * time.Second},
							MaxRPS:        []int{2000},
							ManuallyScale: benchmarks.NewManualScalingConfig("cossim", false, []time.Duration{}, []int{}),
							Autoscale:     &benchmarks.AutoscalingConfig{Scale: false},
						}
					}
					getLeaderCmd := GetCachedScalerClientCmdConstructor(true, len(driverVMs), prewarm, sleep, cacheBenchCfg, cosSimBenchCfg)
					getFollowerCmd := GetCachedScalerClientCmdConstructor(false, len(driverVMs), prewarm, sleep, cacheBenchCfg, cosSimBenchCfg)
					ran := ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
					if oneByOne && ran {
						return
					}
				}
			}
		}
	}
}

// Test Hotel application's tail latency.
func TestHotelMatchTailLatency(t *testing.T) {
	var (
		benchNameBase string = "hotel_match_tail_latency"
		driverVMs     []int  = []int{12} //, 9, 10, 11}
	)
	// Cluster configuration parameters
	var (
		numNodes     int = 12
		numFullNodes int = numNodes
	)
	const (
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
		useGVisor         bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rpsBase     int   = 500 // 95% capacity for a single cossim server
		maxMultiple int   = 2   // max multiple of rpsBase
		rpsMigrate  []int = []int{
			rpsBase * 15,
		}
		durMigrate []time.Duration = []time.Duration{
			5 * time.Second,
		}
		rpsSlow []int = []int{
			rpsBase,
			rpsBase * 2,
		}
		durSlow []time.Duration = []time.Duration{
			10 * time.Second,
			10 * time.Second,
		}
		rpsFast []int = []int{
			// Block 1
			rpsBase,
			rpsBase * maxMultiple,
			rpsBase,
			rpsBase * maxMultiple,
			// Block 2
			rpsBase,
			rpsBase * maxMultiple,
			rpsBase,
			rpsBase * maxMultiple,
			// Block 3
			rpsBase,
			rpsBase * maxMultiple,
			rpsBase,
			rpsBase * maxMultiple,
			// Block 4
			rpsBase,
			rpsBase * maxMultiple,
			rpsBase,
			rpsBase * maxMultiple,
			// Block 5
			rpsBase,
			rpsBase * maxMultiple,
			rpsBase,
			rpsBase * maxMultiple,
			// Finish
			rpsBase,
		}
		durFast []time.Duration = []time.Duration{
			// Block 1
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			// Block 2
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			// Block 3
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			// Block 4
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			// Block 5
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
			// Finish
			100 * time.Millisecond,
		}
		numCaches                    int           = 1
		cacheType                    string        = "cached"
		autoscaleCache               bool          = false
		clientDelay                  time.Duration = 0 * time.Second
		sleep                        time.Duration = 0 * time.Second
		manuallyScaleCaches          bool          = false
		scaleCacheDelay              time.Duration = 0 * time.Second
		numCachesToAdd               int           = 0
		numGeo                       int           = 1
		numGeoIdx                    int           = 1000
		geoSearchRadius              int           = 10
		geoNResults                  int           = 5
		manuallyScaleGeo             bool          = false
		scaleGeoDelay                time.Duration = 0 * time.Second
		numGeoToAdd                  int           = 0
		useCoSandbox                 []bool        = []bool{true, false}
		autoscaleCosSim              bool          = false
		fastLoadChange               []bool        = []bool{false, true}
		proactiveScaling             bool          = true
		cosSimNoCoSandboxScalingTime time.Duration = 85 * time.Millisecond
		cosSimCoSandboxScalingTime   time.Duration = 50 * time.Millisecond
		useMatchCaching              bool          = true
		migrate                      bool          = true
		cachedUserFrac               int64         = 70
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if ts.BCfg.Overlays {
		benchNameBase += "_overlays"
	}
	for _, fast := range fastLoadChange {
		for _, delInit := range useCoSandbox {
			scalingTime := cosSimNoCoSandboxScalingTime
			benchName := benchNameBase
			rps := rpsSlow
			dur := durSlow
			if fast {
				benchName += "_fast"
				rps = rpsFast
				dur = durFast
			}
			nVecToQuery := 5000
			if migrate {
				if fast {
					continue
				}
				benchName += "_migrate"
				rps = rpsMigrate
				dur = durMigrate
				nVecToQuery = 10
			}
			if delInit {
				benchName += "_csdi"
				scalingTime = cosSimCoSandboxScalingTime
			}
			csScaleDurs := make([]time.Duration, len(dur))
			csScaleDeltas := make([]int, len(dur))
			csNSrv := make([]int, len(dur))
			// Calculate the deltas
			for i := range dur {
				if i == 0 {
					csNSrv[i] = 1
				}
				if i < len(dur)-1 {
					if i > 0 {
						csNSrv[i] = csScaleDeltas[i-1] + csNSrv[i-1]
					}
					csScaleDeltas[i] = rps[i+1]/rpsBase - csNSrv[i]
				}
			}
			// Scale up a bit in advance if scaling proactively
			if proactiveScaling {
				for i := range dur {
					// Going to scale up during this period
					if csScaleDeltas[i] > 0 {
						// Scale a bit in advance, and add back the scaling time to the next
						// period to stay in-sync with load shifts
						csScaleDurs[i] = dur[i] - scalingTime
					} else {
						csScaleDurs[i] = 0
					}
				}
			}
			db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
			hotelCfg := &benchmarks.HotelBenchConfig{
				JobCfg: &hotel.HotelJobConfig{
					Job:             "hotel-job",
					Srvs:            hotel.NewHotelSvc(),
					NHotel:          80,
					Cache:           cacheType,
					CacheCfg:        nil,
					ImgSizeMB:       0,
					NGeo:            numGeo,
					NGeoIdx:         numGeoIdx,
					GeoSearchRadius: geoSearchRadius,
					GeoNResults:     geoNResults,
					UseMatch:        true,
				},
				MatchUseCaching: useMatchCaching,
				CachedUserFrac:  cachedUserFrac,
				Durs:            dur,
				MaxRPS:          rps,
				ScaleGeo: &benchmarks.ManualScalingConfig{
					Svc:         "hotel-geo",
					Scale:       manuallyScaleGeo,
					ScaleDelays: []time.Duration{scaleGeoDelay},
					ScaleDeltas: []int{numGeoToAdd},
				},
				CacheBenchCfg: &benchmarks.CacheBenchConfig{
					JobCfg:       &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(4000), GC: true},
					Shmem:        true,
					Autoscale:    autoscaleCache,
					UseCoSandbox: delInit,
					UseEPCache:   true,
					ManuallyScale: &benchmarks.ManualScalingConfig{
						Svc:         "cached",
						Scale:       manuallyScaleCaches,
						ScaleDelays: []time.Duration{scaleCacheDelay},
						ScaleDeltas: []int{numCachesToAdd},
					},
					Migrate: &benchmarks.MigrationConfig{
						Svc:              "cached",
						Migrate:          migrate,
						MigrationDelays:  []time.Duration{2 * time.Second},
						MigrationTargets: []int{0},
					},
				},
				CosSimBenchCfg: &benchmarks.CosSimBenchConfig{
					//					JobCfg:      cossimsrv.NewCosSimJobConfig("hotel-job", 1, 10000, 100, true, 4000, nil, csDelInit),
					JobCfg:      cossimsrv.NewCosSimJobConfig("hotel-job", 1, 4000, 100, true, 4000, nil, delInit),
					NVecToQuery: nVecToQuery,
					ManuallyScale: benchmarks.NewManualScalingConfig("cossim", !autoscaleCosSim,
						csScaleDurs,
						csScaleDeltas,
					),
					Autoscale: &benchmarks.AutoscalingConfig{Svc: "cossim", InitialNReplicas: 1, Scale: autoscaleCosSim, MaxReplicas: 4, TargetRIF: 3, Tolerance: 0.5, Frequency: 10 * time.Millisecond},
				},
			}
			getLeaderCmd := GetHotelClientCmdConstructor("Match", true, len(driverVMs), sleep, hotelCfg)
			getFollowerCmd := GetHotelClientCmdConstructor("Match", false, len(driverVMs), sleep, hotelCfg)
			ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
		}
	}
}

// Test ImgProcess.
func TestImgProcess(t *testing.T) {
	var (
		benchNameBase string = "img_process"
		driverVM      int    = 8
	)
	// Cluster configuration parameters
	var (
		//		numNodes     int = 12
		numNodes     int = 2
		numFullNodes int = numNodes
	)
	const (
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		nrounds        int    = 1
		ntasks         int    = 50
		ninputsPerTask int    = 1
		withGVisor     []bool = []bool{
			false,
			true,
		}
		cosandboxSequential []bool = []bool{
			false,
			true,
		}
		withCoSandbox []bool = []bool{
			false,
			true,
		}
		measurePSS []bool = []bool{
			false,
			true,
		}
		coSandboxWriteOutResult []bool = []bool{
			false,
			true,
		}
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if ts.BCfg.Overlays {
		benchNameBase += "_overlays"
	}
	for _, useGVisor := range withGVisor {
		for _, sequential := range cosandboxSequential {
			for _, cosandbox := range withCoSandbox {
				for _, pss := range measurePSS {
					for _, cosandboxWriteout := range coSandboxWriteOutResult {
						inputPath := "9ps3/img-save/7.jpg"
						//						inputPath := "9ps3/img-save/6.jpg"
						//			inputPath := "9ps3/img-save/1.jpg" // for more I/O-bound version
						benchName := benchNameBase
						if sequential {
							benchName += "_sequential"
						}
						if useGVisor {
							benchName += "_gvisor"
						}
						if cosandbox {
							benchName += "_cosandbox"
						}
						if pss {
							benchName += "_pss"
							// If measuring PSS, only do so when using gVisor, cosandboxes, and
							// sequential execution
							if !useGVisor || !sequential || !cosandbox {
								continue
							}
						}
						if cosandboxWriteout {
							// Only measure cosandbox writing results if not measuring PSS,
							// not running sequentially (running sequentially causes a hang),
							// and if running with cosandboxes
							if sequential || !cosandbox || pss {
								continue
							}
							benchName += "_writeout"
						}
						bsMcpu := proc.Tmcpu(0)
						workerMcpu := proc.Tmcpu(3100)
						imgdMcpu := proc.Tmcpu(1000)
						if sequential {
							bsMcpu = proc.Tmcpu(10)
							workerMcpu = proc.Tmcpu(900)
							imgdMcpu = proc.Tmcpu(50)
						}
						if pss {
							workerMcpu = proc.Tmcpu(3100)
						}
						db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
						imgCfg := &benchmarks.ImgBenchConfig{
							JobCfg: &imgresize.ImgdJobConfig{
								Job:                  "img-job",
								WorkerMcpu:           workerMcpu,
								WorkerMem:            proc.Tmem(0),
								Persist:              false,
								NRounds:              nrounds,
								ImgDim:               160,
								ImgdMcpu:             imgdMcpu,
								UseSPProxy:           true,
								UseCoSandbox:         cosandbox,
								WriteOutViaCoSandbox: cosandboxWriteout,
								UseS3Clnt:            true,
								WorkerCoSandboxMcpu:  bsMcpu,
								WorkerCoSandboxMem:   proc.Tmem(0),
								FTTaskSrvMcpu:        proc.Tmcpu(50),
								PremountS3:           true,
								MeasurePSS:           pss,
							},
							InputPath:      inputPath,
							NTasks:         ntasks,
							NInputsPerTask: ninputsPerTask,
						}
						ts.RunStandardBenchmark(benchName, driverVM, GetImgProcessCmd(imgCfg, useGVisor), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
					}
				}
			}
		}
	}
}

func TestStartLatency(t *testing.T) {
	var (
		benchNameBase string = "start_latency"
		driverVM      int    = 5
	)
	// Cluster configuration parameters
	var (
		numNodes     int = 4
		numFullNodes int = numNodes
	)
	const (
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
	)
	// Benchmark configuration parameters
	var (
		withShmem []bool = []bool{
			false,
			true,
		}
		withCoSandbox []bool = []bool{
			false,
			true,
		}
		// If true, run with gvisor
		apps map[string]bool = map[string]bool{
			"cached":      false,
			"cossim":      false,
			"etcd":        true,
			"memcached":   true,
			"imgrec-py":   false,
			"imgrec-wasm": false,
		}
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if ts.BCfg.Overlays {
		benchNameBase += "_overlays"
	}
	for app, useGVisor := range apps {
		for _, cosandbox := range withCoSandbox {
			for _, shmem := range withShmem {
				benchName := benchNameBase + "_" + app
				if cosandbox {
					benchName += "_cosandbox"
				}
				if !shmem {
					benchName += "_noshmem"
					// Only test cached without shmem
					if app != "cached" {
						continue
					}
				}
				db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
				startLatencyCfg := &benchmarks.StartLatencyBenchConfig{
					App: app,
				}
				// Create default configs for each app
				cacheBenchCfg := &benchmarks.CacheBenchConfig{
					JobCfg: &cachegrpmgr.CacheJobConfig{
						NSrv: 1,
						MCPU: proc.Tmcpu(4000),
						GC:   true,
					},
					CPP:          true,
					Shmem:        shmem,
					UseEPCache:   true,
					UseCoSandbox: cosandbox,
					Autoscale:    false,
					NKeys:        3000,
					ValSize:      5000,
					TopNShards:   0,
					ManuallyScale: &benchmarks.ManualScalingConfig{
						Svc:         "cached",
						Scale:       true,
						ScaleDelays: []time.Duration{5 * time.Second},
						ScaleDeltas: []int{1},
					},
					Migrate: &benchmarks.MigrationConfig{
						Svc:              "cached",
						Migrate:          false,
						MigrationDelays:  []time.Duration{},
						MigrationTargets: []int{},
					},
				}
				cossimCfg := &benchmarks.CosSimBenchConfig{
					JobCfg: &cossimsrv.CosSimJobConfig{
						Job:       "cossim-job",
						InitNSrv:  1,
						NVec:      9000,
						VecDim:    128,
						EagerInit: true,
						SrvMcpu:   proc.Tmcpu(4000),
						CacheCfg: &cachegrpmgr.CacheJobConfig{
							NSrv: 1,
							MCPU: proc.Tmcpu(1000),
							GC:   true,
						},
						UseCoSandboxRPCs: cosandbox,
					},
				}
				etcdCfg := &benchmarks.EtcdBenchConfig{
					JobCfg: &etcd.EtcdJobConfig{
						Job:            "etcd-job",
						SnapshotS3Path: "name/s3/~local/9ps3/snapshot-14MB.db",
						SnapshotUXPath: "name/ux/~local/snapshot-14MB.db",
						UseUX:          false,
						Name:           "etcd-proc",
						PeerPort:       6380,
						ClientPort:     6379,
						UseCoSandbox:   cosandbox,
						Mcpu:           proc.Tmcpu(4000),
						ShmemMB:        proc.Tmem(250),
					},
				}
				memcachedCfg := &benchmarks.MemcachedBenchConfig{
					JobCfg: &memcached.MemcachedJobConfig{
						Job:            "memcached-job",
						SnapshotS3Path: "name/s3/~local/9ps3/memcached-snapshot-200M",
						SnapshotUXPath: "name/ux/~local/memcached-snapshot-200M",
						UseUX:          true,
						Port:           11211,
						UseCoSandbox:   cosandbox,
						Mcpu:           proc.Tmcpu(4000),
						ShmemMB:        proc.Tmem(250),
					},
					Cache: false,
				}
				imgrecPyShmemMB := proc.Tmem(256)
				imgrecPyCfg := &benchmarks.ImgrecPyBenchConfig{
					JobCfg: &imgrec_py.ImgrecPyJobConfig{
						ImgBucket:    "9ps3",
						ImgKey:       "img-save/8.jpg",
						ModelBucket:  "9ps3",
						ModelKey:     "mobilenetv2-12.onnx",
						Kid:          "~local",
						UseCoSandbox: cosandbox,
						AsyncFetch:   true,
						ShmemMB:      imgrecPyShmemMB,
						Mcpu:         proc.Tmcpu(4000),
					},
				}
				imgrecWASMShmemMB := proc.Tmem(256)
				imgrecWASMCfg := &benchmarks.ImgrecWASMBenchConfig{
					JobCfg: &imgrec_wasm.ImgrecWASMJobConfig{
						ImgBucket:         "9ps3",
						ImgKey:            "img-save/8.jpg",
						ModelBucket:       "9ps3",
						ModelKey:          "mobilenetv2-12.onnx",
						Kid:               "~local",
						UseDelegated:      cosandbox,
						UseCoSandbox:      cosandbox,
						ShmemMB:           imgrecWASMShmemMB,
						UseWriteReadShmem: cosandbox,
						Mcpu:              proc.Tmcpu(4000),
					},
				}
				cmdFn := GetStartLatencyCmdConstructor(startLatencyCfg, cacheBenchCfg, cossimCfg, etcdCfg, memcachedCfg, imgrecPyCfg, imgrecWASMCfg, cosandbox, useGVisor)
				ts.RunStandardBenchmark(benchName, driverVM, cmdFn, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, useGVisor)
			}
		}
	}
}

func TestSebsStartLatency(t *testing.T) {
	var (
		benchNameBase string = "sebs_start_latency"
		driverVM      int    = 5
	)
	// Cluster configuration parameters
	const (
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
	)
	var (
		numNodes     int = 4
		numFullNodes int = numNodes
	)
	type benchSpec struct {
		defaultCfg        *benchmarks.SebsBenchConfig
		useGVisor         bool
		supportsCosandbox bool
	}
	// All benchmarks run without gVisor.
	benchSpecs := []benchSpec{
		{benchmarks.DefaultSebsThumbnailerBenchConfig, false, true},
		{benchmarks.DefaultSebsVideoProcessingBenchConfig, false, true},
		{benchmarks.DefaultSebsImageRecognitionBenchConfig, false, true},
		{benchmarks.DefaultSebsDnaVisualisationBenchConfig, false, true},
		{benchmarks.DefaultSebsSleepBenchConfig, false, false},
		{benchmarks.DefaultSebsDynamicHtmlBenchConfig, false, false},
		{benchmarks.DefaultSebsUploaderBenchConfig, false, false},
		{benchmarks.DefaultSebsGraphPagerankBenchConfig, false, false},
		{benchmarks.DefaultSebsGraphMstBenchConfig, false, false},
		{benchmarks.DefaultSebsGraphBfsBenchConfig, false, false},
	}
	withCoSandbox := []bool{false, true}
	withUncompressed := []bool{false}

	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	if ts.BCfg.Overlays {
		benchNameBase += "_overlays"
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, spec := range benchSpecs {
		for _, cosandbox := range withCoSandbox {
			for _, uncompressed := range withUncompressed {
				// Uncompressed bundles are only supported on the plain (non-cosandbox) path.
				if cosandbox && uncompressed {
					continue
				}
				benchName := benchNameBase + "_" + spec.defaultCfg.Benchmark
				if cosandbox {
					benchName += "_cosandbox"
				}
				if uncompressed {
					benchName += "_uncompressed"
				}
				cfg := *spec.defaultCfg
				if spec.supportsCosandbox {
					cfg.UseCoSandbox = cosandbox
					cfg.AsyncFetch = !cosandbox
				}
				cfg.Uncompressed = uncompressed
				cmdFn := GetSebsStartLatencyCmdConstructor(&cfg, spec.useGVisor)
				ts.RunStandardBenchmark(benchName, driverVM, cmdFn, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost, spec.useGVisor)
			}
		}
	}
}
