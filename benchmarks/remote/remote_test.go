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
	"sigmaos/apps/hotel"
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
	flag.BoolVar(&k8sArg, "k8s", false, "Run the k8s version of the experiment.")
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
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	ts.RunStandardBenchmark(benchName, driverVM, GetInitFSCmd, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
	ts.RunStandardBenchmark(benchName, driverVM, getExampleCmd, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
	ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(rps, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
			ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, nCores, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, dummyProc, lcProc, prewarmRealm, skipStats), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
	}
}

// Run the SigmaOS MapReduce benchmark
func TestMR(t *testing.T) {
	var (
		benchNameBase string = "mr_vs_corral"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 0
		numProcqOnlyNodes int  = 1
		turboBoost        bool = true
	)
	type MRExperimentConfig struct {
		benchName       string
		numNodes        int
		numCoresPerNode uint
		memReq          proc.Tmem
	}
	// Variable MR benchmark configuration parameters
	var (
		mrApps []*MRExperimentConfig = []*MRExperimentConfig{
			{"mr-grep-wiki2G-bench-s3.yml", 10, 4, 7000},
			{"mr-grep-wiki2G-granular-bench-s3.yml", 54, 4, 7000},
			{"mr-wc-wiki2G-bench.yml", 10, 4, 7000},
			{"mr-wc-wiki2G-bench-s3.yml", 10, 4, 7000},
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
				benchName := filepath.Join(benchNameBase, mrEP.benchName)
				if prewarmRealm {
					benchName += "-warm"
				} else {
					benchName += "-cold"
				}
				if perf {
					benchName += "-perf"
				}
				numFullNodes := mrEP.numNodes - numProcqOnlyNodes
				ts.RunStandardBenchmark(benchName, driverVM, GetMRCmdConstructor(mrEP.benchName, mrEP.memReq, prewarmRealm, measureTpt, perf), mrEP.numNodes, mrEP.numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
	)
	// Variable MR benchmark configuration parameters
	var (
		corralApps []string = []string{"corral-2G-cold", "corral-2G-warm"}
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, corralApp := range corralApps {
		benchName := filepath.Join(benchNameBase, corralApp)
		ts.RunStandardBenchmark(benchName, driverVM, GetCorralCmdConstructor(), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		Durs:   dur,
		MaxRPS: rps,
		ScaleGeo: &benchmarks.ManualScalingConfig{
			Svc:         "hotel-geo",
			Scale:       manuallyScaleGeo,
			ScaleDelays: []time.Duration{scaleGeoDelay},
			ScaleDeltas: []int{numGeoToAdd},
		},
		CacheBenchCfg: &benchmarks.CacheBenchConfig{
			JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
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
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
					Durs:   dur,
					MaxRPS: rps,
					ScaleGeo: &benchmarks.ManualScalingConfig{
						Svc:         "hotel-geo",
						Scale:       scale,
						ScaleDelays: []time.Duration{scaleGeoDelay},
						ScaleDeltas: []int{numGeoToAdd},
					},
					CacheBenchCfg: &benchmarks.CacheBenchConfig{
						JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
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
				ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
					Durs:   dur,
					MaxRPS: rps,
					ScaleGeo: &benchmarks.ManualScalingConfig{
						Svc:         "hotel-geo",
						Scale:       scale,
						ScaleDelays: []time.Duration{scaleGeoDelay},
						ScaleDeltas: []int{numGeoToAdd},
					},
					CacheBenchCfg: &benchmarks.CacheBenchConfig{
						JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
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
				ran := ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
					Durs:   dur,
					MaxRPS: rps,
					ScaleGeo: &benchmarks.ManualScalingConfig{
						Svc:         "hotel-geo",
						Scale:       manuallyScaleGeo,
						ScaleDelays: []time.Duration{scaleGeoDelay},
						ScaleDeltas: []int{numGeoToAdd},
					},
					CacheBenchCfg: &benchmarks.CacheBenchConfig{
						JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
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
				ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sSocialnetApp, stopK8sSocialnetApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
}

// Test multiplexing Best Effort ImgResize jobs.
func TestBEImgResizeMultiplexing(t *testing.T) {
	var (
		benchName string = "be_imgresize_multiplexing"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 0
		numNodes          int  = 8 // 24
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 0
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	ts.RunStandardBenchmark(benchName, driverVM, GetBEImgResizeMultiplexingCmd, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
}

// Test multiplexing Best Effort ImgResize jobs.
func TestBEImgResizeRPCMultiplexing(t *testing.T) {
	var (
		benchName string = "be_imgresize_rpc_multiplexing"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 0
		numNodes          int  = 26
		numCoresPerNode   uint = 4
		numProcqOnlyNodes int  = 2
		numFullNodes      int  = numNodes - numProcqOnlyNodes
		turboBoost        bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	ts.RunStandardBenchmark(benchName, driverVM, GetBEImgResizeRPCMultiplexingCmd, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		Durs:   dur,
		MaxRPS: rps,
		ScaleGeo: &benchmarks.ManualScalingConfig{
			Svc:         "hotel-geo",
			Scale:       manuallyScaleGeo,
			ScaleDelays: []time.Duration{scaleGeoDelay},
			ScaleDeltas: []int{numGeoToAdd},
		},
		CacheBenchCfg: &benchmarks.CacheBenchConfig{
			JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
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
	getLeaderCmd := GetLCBEHotelImgResizeMultiplexingCmdConstructor(len(driverVMs), rps, dur, cacheType, autoscaleCache, sleep)
	getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), sleep, hotelCfg)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, nil, nil, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		Durs:   dur,
		MaxRPS: rps,
		ScaleGeo: &benchmarks.ManualScalingConfig{
			Svc:         "hotel-geo",
			Scale:       manuallyScaleGeo,
			ScaleDelays: []time.Duration{scaleGeoDelay},
			ScaleDeltas: []int{numGeoToAdd},
		},
		CacheBenchCfg: &benchmarks.CacheBenchConfig{
			JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(2000), GC: true},
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
	getLeaderCmd := GetLCBEHotelImgResizeRPCMultiplexingCmdConstructor(len(driverVMs), rps, dur, cacheType, autoscaleCache, sleep)
	getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), sleep, hotelCfg)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, nil, nil, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
	)
	// CosSim benchmark configuration parameters
	var (
		numCosSimBase         int             = 1
		nCache                []int           = []int{1, 2, 4}
		clientDelay           time.Duration   = 0 * time.Second
		sleep                 time.Duration   = 0 * time.Second
		delegateInit          []bool          = []bool{true, false}
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
		for _, delegate := range delegateInit {
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
						ran := ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
	)
	// Cached benchmark configuration parameters
	var (
		clientDelay      time.Duration = 0 * time.Second
		sleep            time.Duration = 0 * time.Second
		delegateInit     []bool        = []bool{true, false}
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
			for _, delegate := range delegateInit {
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
						CosSimBackend: cossimBackend,
						UseEPCache:    useEPCache,
						DelegateInit:  delegate,
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
					ran := ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
	)
	// Hotel benchmark configuration parameters
	var (
		rpsBase     int   = 500 // 95% capacity for a single cossim server
		maxMultiple int   = 2   // max multiple of rpsBase
		rpsSlow     []int = []int{
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
		numCaches                        int           = 1
		cacheType                        string        = "cached"
		autoscaleCache                   bool          = false
		clientDelay                      time.Duration = 0 * time.Second
		sleep                            time.Duration = 0 * time.Second
		manuallyScaleCaches              bool          = false
		scaleCacheDelay                  time.Duration = 0 * time.Second
		numCachesToAdd                   int           = 0
		numGeo                           int           = 1
		numGeoIdx                        int           = 1000
		geoSearchRadius                  int           = 10
		geoNResults                      int           = 5
		manuallyScaleGeo                 bool          = false
		scaleGeoDelay                    time.Duration = 0 * time.Second
		numGeoToAdd                      int           = 0
		cosSimDelegatedInit              []bool        = []bool{true, false}
		autoscaleCosSim                  bool          = false
		fastLoadChange                   []bool        = []bool{false, true}
		proactiveScaling                 bool          = true
		cosSimNoDelegatedInitScalingTime time.Duration = 85 * time.Millisecond
		cosSimDelegatedInitScalingTime   time.Duration = 50 * time.Millisecond
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if ts.BCfg.Overlays {
		benchNameBase += "_overlays"
	}
	for _, fast := range fastLoadChange {
		for _, csDelInit := range cosSimDelegatedInit {
			scalingTime := cosSimNoDelegatedInitScalingTime
			benchName := benchNameBase
			rps := rpsSlow
			dur := durSlow
			if fast {
				benchName += "_fast"
				rps = rpsFast
				dur = durFast
			}
			if csDelInit {
				benchName += "_csdi"
				scalingTime = cosSimDelegatedInitScalingTime
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
				MatchUseCaching: false,
				Durs:            dur,
				MaxRPS:          rps,
				ScaleGeo: &benchmarks.ManualScalingConfig{
					Svc:         "hotel-geo",
					Scale:       manuallyScaleGeo,
					ScaleDelays: []time.Duration{scaleGeoDelay},
					ScaleDeltas: []int{numGeoToAdd},
				},
				CacheBenchCfg: &benchmarks.CacheBenchConfig{
					JobCfg:    &cachegrpmgr.CacheJobConfig{NSrv: numCaches, MCPU: proc.Tmcpu(4000), GC: true},
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
				CosSimBenchCfg: &benchmarks.CosSimBenchConfig{
					JobCfg:      cossimsrv.NewCosSimJobConfig("hotel-job", 1, 10000, 100, true, 4000, nil, csDelInit),
					NVecToQuery: 5000,
					ManuallyScale: benchmarks.NewManualScalingConfig("cossim", !autoscaleCosSim,
						csScaleDurs,
						csScaleDeltas,
					),
					Autoscale: &benchmarks.AutoscalingConfig{Svc: "cossim", InitialNReplicas: 1, Scale: autoscaleCosSim, MaxReplicas: 4, TargetRIF: 3, Tolerance: 0.5, Frequency: 10 * time.Millisecond},
				},
			}
			getLeaderCmd := GetHotelClientCmdConstructor("Match", true, len(driverVMs), sleep, hotelCfg)
			getFollowerCmd := GetHotelClientCmdConstructor("Match", false, len(driverVMs), sleep, hotelCfg)
			ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
		}
	}
}
