package remote

import (
	"flag"
	"testing"

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
		driverVM        int  = 0
		numNodes        int  = 4
		numCoresPerNode uint = 4
		onlyOneFullNode bool = false
		turboBoost      bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark:\n%v", ts)
	ts.RunStandardBenchmark(benchName, driverVM, GetInitFSCmd, numNodes, numCoresPerNode, onlyOneFullNode, turboBoost)
}

// Test SigmaOS cold-start.
func TestColdStart(t *testing.T) {
	var (
		benchName string = "cold_start"
	)
	// Cluster configuration parameters
	const (
		driverVM        int  = 7
		numNodes        int  = 8
		numCoresPerNode uint = 16
		onlyOneFullNode bool = true
		turboBoost      bool = true
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark:\n%v", ts)
	ts.RunStandardBenchmark(benchName, driverVM, GetColdStartCmd, numNodes, numCoresPerNode, onlyOneFullNode, turboBoost)
}

// Run the SigmaOS MapReduce benchmark
func TestMR(t *testing.T) {
	var (
		benchNameBase string = "mr_vs_corral"
	)
	// Cluster configuration parameters
	const (
		driverVM        int  = 0
		numNodes        int  = 8
		numCoresPerNode uint = 2
		onlyOneFullNode bool = false
		turboBoost      bool = true
	)
	// Variable MR benchmark configuration parameters
	var (
		mrApps        []string = []string{"mr-wc-wiki2G-bench.yml", "mr-wc-wiki2G-bench-s3.yml"}
		prewarmRealms []bool   = []bool{false, true}
	)
	// Constant MR benchmark configuration parameters
	const (
		memReq     proc.Tmem = 7000
		asyncRW    bool      = true
		measureTpt bool      = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	db.DPrintf(db.ALWAYS, "Benchmark:\n%v", ts)
	for _, mrApp := range mrApps {
		for _, prewarmRealm := range prewarmRealms {
			benchName := benchNameBase
			if prewarmRealm {
				benchName += "-warm"
			} else {
				benchName += "-cold"
			}
			ts.RunStandardBenchmark(benchName, driverVM, GetMRCmdConstructor(mrApp, memReq, asyncRW, prewarmRealm, measureTpt), numNodes, numCoresPerNode, onlyOneFullNode, turboBoost)
		}
	}
}
