package remote

import (
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

var platformArg string
var vpcArg string
var tagArg string
var branchArg string
var versionArg string
var noNetproxyArg bool
var overlaysArg bool
var parallelArg bool
var noShutdownArg bool
var k8sArg bool
var oneByOne bool

func init() {
	flag.StringVar(&platformArg, "platform", sp.NOT_SET, "Platform on which to run. Currently, only [aws|cloudlab] are supported")
	flag.StringVar(&vpcArg, "vpc", sp.NOT_SET, "VPC in which to run. Need not be specified for Cloudlab.")
	flag.StringVar(&tagArg, "tag", sp.NOT_SET, "Build tag with which to run.")
	flag.StringVar(&branchArg, "branch", "master", "Branch on which to run.")
	flag.StringVar(&versionArg, "version", sp.NOT_SET, "Output version string.")
	flag.BoolVar(&noNetproxyArg, "nodialproxy", false, "Disable use of proxy for network dialing/listening.")
	flag.BoolVar(&overlaysArg, "overlays", false, "Run with Docker swarm overlays enabled.")
	flag.BoolVar(&parallelArg, "parallelize", false, "Run commands in parallel to speed up, e.g., cluster shutdown.")
	flag.BoolVar(&oneByOne, "one-by-one", false, "Run one benchmark part, and then return")
	flag.BoolVar(&noShutdownArg, "no-shutdown", false, "Avoid shutting down the cluster after running a benchmark (useful for debugging).")
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
		numNodes          int  = 4
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
		driverVM          int  = 3
		numNodes          int  = 2
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
		dummyProc     bool          = false
		lcProc        bool          = false
		prewarmRealm  bool          = true
		skipStats     bool          = true
		rps           []int         = []int{1600, 1200, 800, 400}
		nCoresPerNode []uint        = []uint{40, 32, 16, 8, 4, 2}
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
		numProcqOnlyNodes int  = 2
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
			&MRExperimentConfig{"mr-grep-wiki2G-bench-s3.yml", 10, 4, 7000},
			&MRExperimentConfig{"mr-grep-wiki2G-granular-bench-s3.yml", 54, 4, 7000},
			&MRExperimentConfig{"mr-wc-wiki2G-bench.yml", 10, 4, 7000},
			&MRExperimentConfig{"mr-wc-wiki2G-bench-s3.yml", 10, 4, 7000},
		}
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
	for _, mrEP := range mrApps {
		for _, prewarmRealm := range prewarmRealms {
			benchName := filepath.Join(benchNameBase, mrEP.benchName)
			if prewarmRealm {
				benchName += "-warm"
			} else {
				benchName += "-cold"
			}
			numFullNodes := mrEP.numNodes - numProcqOnlyNodes
			ts.RunStandardBenchmark(benchName, driverVM, GetMRCmdConstructor(mrEP.benchName, mrEP.memReq, prewarmRealm, measureTpt), mrEP.numNodes, mrEP.numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		scaleCache          bool            = false
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
	getLeaderCmd := GetHotelClientCmdConstructor("Search", true, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, manuallyScaleCaches, scaleCacheDelay, numCachesToAdd, numGeo, numGeoIdx, geoSearchRadius, geoNResults, manuallyScaleGeo, scaleGeoDelay, numGeoToAdd)
	getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, manuallyScaleCaches, scaleCacheDelay, numCachesToAdd, numGeo, numGeoIdx, geoSearchRadius, geoNResults, manuallyScaleGeo, scaleGeoDelay, numGeoToAdd)
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
		scaleCache         bool            = false
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
				getLeaderCmd := GetHotelClientCmdConstructor("Search", true, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, false, 0, 0, numGeo, numGeoIdx, geoSearchRadius, geoNResults, scale, scaleGeoDelay, numGeoToAdd)
				getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, false, 0, 0, numGeo, numGeoIdx, geoSearchRadius, geoNResults, scale, scaleGeoDelay, numGeoToAdd)
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
		scaleCache         bool            = false
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
				getLeaderCmd := GetHotelClientCmdConstructor("Geo", true, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, false, 0, 0, numGeo, numGeoIdx, geoSearchRadius, geoNResults, scale, scaleGeoDelay, numGeoToAdd)
				getFollowerCmd := GetHotelClientCmdConstructor("Geo", false, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, false, 0, 0, numGeo, numGeoIdx, geoSearchRadius, geoNResults, scale, scaleGeoDelay, numGeoToAdd)
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
		scaleCache           bool            = false
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
				getLeaderCmd := GetHotelClientCmdConstructor("Search", true, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, scale, scaleCacheDelay, numCachesToAdd, numGeo, numGeoIdx, geoSearchRadius, geoNResults, manuallyScaleGeo, scaleGeoDelay, numGeoToAdd)
				getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, scale, scaleCacheDelay, numCachesToAdd, numGeo, numGeoIdx, geoSearchRadius, geoNResults, manuallyScaleGeo, scaleGeoDelay, numGeoToAdd)
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
		scaleCache          bool            = false
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
	getLeaderCmd := GetLCBEHotelImgResizeMultiplexingCmdConstructor(len(driverVMs), rps, dur, cacheType, scaleCache, sleep)
	getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, manuallyScaleCaches, scaleCacheDelay, numCachesToAdd, numGeo, numGeoIdx, geoSearchRadius, geoNResults, manuallyScaleGeo, scaleGeoDelay, numGeoToAdd)
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
		scaleCache          bool            = false
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
	getLeaderCmd := GetLCBEHotelImgResizeRPCMultiplexingCmdConstructor(len(driverVMs), rps, dur, cacheType, scaleCache, sleep)
	getFollowerCmd := GetHotelClientCmdConstructor("Search", false, len(driverVMs), rps, dur, numCaches, cacheType, scaleCache, sleep, manuallyScaleCaches, scaleCacheDelay, numCachesToAdd, numGeo, numGeoIdx, geoSearchRadius, geoNResults, manuallyScaleGeo, scaleGeoDelay, numGeoToAdd)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, nil, nil, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
}

// Test CosSim Geo's application tail latency.
func TestScaleCosSim(t *testing.T) {
	var (
		benchNameBase string = "cos_sim_tail_latency"
		driverVMs     []int  = []int{9}
	)
	// Cluster configuration parameters
	const (
		numNodes          int  = 4
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
	)
	// CosSim benchmark configuration parameters
	var (
		//		rps                   []int           = []int{300, 600, 600}
		//		dur                   []time.Duration = []time.Duration{5 * time.Second, 30 * time.Second, 30 * time.Second}
		rps                   []int           = []int{600, 650, 1000} //1300}
		dur                   []time.Duration = []time.Duration{5 * time.Second, 30 * time.Second, 30 * time.Second}
		numCosSimBase         int             = 1
		numCaches             int             = 1
		scaleCache            bool            = false
		clientDelay           time.Duration   = 0 * time.Second
		sleep                 time.Duration   = 0 * time.Second
		nvec                  int             = 5000
		vecDim                int             = 100
		eagerInit             []bool          = []bool{true, false}
		manuallyScaleCosSim   []bool          = []bool{true, false}
		scaleCosSimDelayBase  time.Duration   = 35 * time.Second
		scaleCosSimExtraDelay []time.Duration = []time.Duration{0}
		nAdditionalCosSim     []int           = []int{0, 1}
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	for _, eager := range eagerInit {
		for _, scale := range manuallyScaleCosSim {
			for _, numCosSimToAdd := range nAdditionalCosSim {
				for _, extraDelay := range scaleCosSimExtraDelay {
					// Don't add artificial delays for k8s
					if ts.BCfg.K8s {
						extraDelay = 0
					}
					db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
					benchName := benchNameBase
					numCosSim := numCosSimBase
					scaleCosSimDelay := scaleCosSimDelayBase
					if eager {
						benchName += "_eager"
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
						numCosSim += numCosSimToAdd
						benchName += "_no_scale_cossim_nsrv_" + strconv.Itoa(numCosSim)
					}
					getLeaderCmd := GetCosSimClientCmdConstructor("CosSim", true, len(driverVMs), rps, dur, numCaches, scaleCache, sleep, false, 0, 0, numCosSim, nvec, vecDim, eager, scale, scaleCosSimDelay, numCosSimToAdd)
					getFollowerCmd := GetCosSimClientCmdConstructor("CosSim", false, len(driverVMs), rps, dur, numCaches, scaleCache, sleep, false, 0, 0, numCosSim, nvec, vecDim, eager, scale, scaleCosSimDelay, numCosSimToAdd)
					ran := ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
					if oneByOne && ran {
						return
					}
				}
			}
		}
	}
}
