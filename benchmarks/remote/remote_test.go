package remote

import (
	"flag"
	"fmt"
	"path/filepath"
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

func init() {
	flag.StringVar(&platformArg, "platform", sp.NOT_SET, "Platform on which to run. Currently, only [aws|cloudlab] are supported")
	flag.StringVar(&vpcArg, "vpc", sp.NOT_SET, "VPC in which to run. Need not be specified for Cloudlab.")
	flag.StringVar(&tagArg, "tag", sp.NOT_SET, "Build tag with which to run.")
	flag.StringVar(&branchArg, "branch", "master", "Branch on which to run.")
	flag.StringVar(&versionArg, "version", sp.NOT_SET, "Output version string.")
	flag.BoolVar(&noNetproxyArg, "nonetproxy", false, "Disable use of proxy for network dialing/listening.")
	flag.BoolVar(&overlaysArg, "overlays", false, "Run with Docker swarm overlays enabled.")
	flag.BoolVar(&parallelArg, "parallelize", false, "Run commands in parallel to speed up, e.g., cluster shutdown.")
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
		rps int           = 8
		dur time.Duration = 5 * time.Second
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if !assert.False(ts.t, ts.BCfg.K8s, "K8s version of benchmark does not exist") {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(rps, dur, false, false), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
}

// Test the single-node proc start bottleneck.
func TestSingleMachineMaxTpt(t *testing.T) {
	var (
		benchNameBase string = "single_machine_max_start_tpt"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 1
		numNodes          int  = 1
		numFullNodes      int  = 1
		numProcqOnlyNodes int  = 0
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
		rpsPerCore    []int         = []int{100, 250}
		nCoresPerNode []uint        = []uint{2, 4, 8, 16, 32, 40} // In practice, scaling stops well before we reach 32 cores
		dur           time.Duration = 5 * time.Second
	)
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, nCores := range nCoresPerNode {
		for _, perCoreRPS := range rpsPerCore {
			rps := int(nCores) * perCoreRPS
			benchName := filepath.Join(benchNameBase, fmt.Sprintf("%v-cores-rps-%v", nCores, rps))
			ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(rps, dur, false, true), numNodes, nCores, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		driverVM          int  = 24
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
		rps []int         = []int{4600, 9200, 13800, 18400, 23000, 27600, 32200, 36800, 41400, 46000}
		dur time.Duration = 5 * time.Second
	)
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, r := range rps {
		benchName := filepath.Join(benchNameBase, fmt.Sprintf("%v-vm-rps-%v", numNodes, r))
		ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, true, true), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
	}
}

// Test the maximum throughput of a single procq.
func TestSchedProcqMaxTpt(t *testing.T) {
	var (
		benchNameBase string = "procq_max_tpt"
	)
	// Cluster configuration parameters
	const (
		driverVM          int  = 24
		numNodes          int  = 24
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
		rps []int         = []int{18400, 23000, 27600, 32200, 36800, 41400, 46000}
		dur time.Duration = 5 * time.Second
	)
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, r := range rps {
		benchName := filepath.Join(benchNameBase, fmt.Sprintf("%v-vm-rps-%v", numNodes, r))
		ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, true, true), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		numNodes          int  = 24
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
		rps []int         = []int{27600, 32200, 36800, 41400, 46000}
		dur time.Duration = 5 * time.Second
	)
	db.DPrintf(db.ALWAYS, "Benchmark configuration:\n%v", ts)
	for _, r := range rps {
		benchName := filepath.Join(benchNameBase, fmt.Sprintf("%v-vm-rps-%v", numNodes, r))
		ts.RunStandardBenchmark(benchName, driverVM, GetStartCmdConstructor(r, dur, false, true), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		numNodes          int  = 8
		numCoresPerNode   uint = 2
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = true
	)
	// Variable MR benchmark configuration parameters
	var (
		mrApps        []string = []string{"mr-wc-wiki2G-granular-bench.yml", "mr-wc-wiki2G-granular-bench-s3.yml", "mr-wc-wiki2G-bench.yml", "mr-wc-wiki2G-bench-s3.yml"}
		prewarmRealms []bool   = []bool{false, true}
	)
	// Constant MR benchmark configuration parameters
	const (
		memReq proc.Tmem = 1700
		//		memReq     proc.Tmem = 7000
		asyncRW    bool = true
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
	for _, mrApp := range mrApps {
		for _, prewarmRealm := range prewarmRealms {
			benchName := filepath.Join(benchNameBase, mrApp)
			if prewarmRealm {
				benchName += "-warm"
			} else {
				benchName += "-cold"
			}
			ts.RunStandardBenchmark(benchName, driverVM, GetMRCmdConstructor(mrApp, memReq, asyncRW, prewarmRealm, measureTpt), numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
	const (
		numNodes          int  = 8
		numCoresPerNode   uint = 4
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rps         []int           = []int{250, 500, 1000, 1500, 2000, 2500}
		dur         []time.Duration = []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second}
		cacheType   string          = "cached"
		scaleCache  bool            = false
		clientDelay time.Duration   = 60 * time.Second
		sleep       time.Duration   = 10 * time.Second
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
	getLeaderCmd := GetHotelClientCmdConstructor(true, len(driverVMs), rps, dur, cacheType, scaleCache, sleep)
	getFollowerCmd := GetHotelClientCmdConstructor(false, len(driverVMs), rps, dur, cacheType, scaleCache, sleep)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, startK8sHotelApp, stopK8sHotelApp, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
		rps         []int           = []int{250, 500, 1000, 1500, 2000, 1000}
		dur         []time.Duration = []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 20 * time.Second, 15 * time.Second}
		cacheType   string          = "cached"
		scaleCache  bool            = false
		clientDelay time.Duration   = 60 * time.Second
		sleep       time.Duration   = 10 * time.Second
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
	getFollowerCmd := GetHotelClientCmdConstructor(false, len(driverVMs), rps, dur, cacheType, scaleCache, sleep)
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
		numFullNodes      int  = numNodes
		numProcqOnlyNodes int  = 0
		turboBoost        bool = false
	)
	// Hotel benchmark configuration parameters
	var (
		rps         []int           = []int{250, 500, 1000, 1500, 2000, 1000}
		dur         []time.Duration = []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 20 * time.Second, 15 * time.Second}
		cacheType   string          = "cached"
		scaleCache  bool            = false
		clientDelay time.Duration   = 60 * time.Second
		sleep       time.Duration   = 10 * time.Second
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
	getFollowerCmd := GetHotelClientCmdConstructor(false, len(driverVMs), rps, dur, cacheType, scaleCache, sleep)
	ts.RunParallelClientBenchmark(benchName, driverVMs, getLeaderCmd, getFollowerCmd, nil, nil, clientDelay, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
}
