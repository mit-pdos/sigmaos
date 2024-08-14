package remote

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
)

type GetBenchCmdFn func(*BenchConfig, *ClusterConfig) string

type Tstate struct {
	t    *testing.T
	BCfg *BenchConfig   `json:"config"`
	LCfg *LocalFSConfig `json:"local_fs_cfg"`
}

func NewTstate(t *testing.T) (*Tstate, error) {
	ts := &Tstate{
		t: t,
	}
	cfg, err := NewBenchConfig()
	if err != nil {
		return ts, err
	}
	ts.BCfg = cfg
	lcfg, err := NewLocalFSConfig(ts.BCfg.Platform, ts.BCfg.Version, ts.BCfg.Parallel)
	if err != nil {
		return ts, err
	}
	ts.LCfg = lcfg
	return ts, nil
}

// Run a standard benchmark:
// 1. Create dirs to hold benchmark output
// 2. Stop running SigmaOS clusters
// 3. Start a SigmaOS cluster
// 4. Run the benchmark
// 5. Collect benchmark results
// 6. Stop the SigmaOS cluster
//
// If any of the above steps results in an error, bail out early.
func (ts *Tstate) RunStandardBenchmark(benchName string, driverVM int, getBenchCmd GetBenchCmdFn, numNodes int, numCoresPerNode uint, onlyOneFullNode bool, turboBoost bool) {
	// Set up the benchmark, and bail out if the benchmark already ran
	if alreadyRan, err := ts.PrepareToRunBenchmark(benchName); !assert.Nil(ts.t, err, "Prepare benchmark: %v", err) {
		return
	} else if alreadyRan {
		db.DPrintf(db.ALWAYS, "========== Skipping %v (already ran) ==========", benchName)
		return
	}
	// First, stop any previously running cluster
	if err := ts.StopSigmaOSCluster(); !assert.Nil(ts.t, err, "Stop cluster: %v", err) {
		return
	}
	// Start a SigmaOS cluster
	ccfg, err := ts.StartSigmaOSCluster(numNodes, numCoresPerNode, onlyOneFullNode, turboBoost)
	db.DPrintf(db.ALWAYS, "\nCluster config:\n%v", ccfg)
	if !assert.Nil(ts.t, err, "Start cluster: %v", err) {
		return
	}
	// Optionally skip shutting down the cluster after the benchmark completes
	// (useful for debugging)
	if !ts.BCfg.NoShutdown {
		defer func() {
			// Stop the SigmaOS cluster once the benchmark is over
			err := ts.StopSigmaOSCluster()
			assert.Nil(ts.t, err, "Stop cluster: %v", err)
		}()
	}
	// Run the benchmark
	err = ccfg.RunBenchmark(driverVM, getBenchCmd(ts.BCfg, ccfg))
	assert.Nil(ts.t, err, "Run benchmark: %v", err)
	// Collect the benchmark results
	if err := ccfg.CollectResults(benchName); !assert.Nil(ts.t, err, "Stop cluster: %v", err) {
		return
	}
}

// Prepare to run a benchmark. If the benchmark has already been run, skip
// preparation and return true.
func (ts *Tstate) PrepareToRunBenchmark(benchName string) (bool, error) {
	outDirPath := ts.LCfg.GetOutputDirPath(benchName)
	// Output for benchmark already exists. Skip running it again.
	if ts.LCfg.OutputExists(outDirPath) {
		return true, nil
	}
	// Create a directory to hold the benchmark's output
	if err := ts.LCfg.CreateOutputDir(outDirPath); err != nil {
		return false, err
	}
	return false, nil
}

// Start a SigmaOS cluster
func (ts *Tstate) StartSigmaOSCluster(numNodes int, numCoresPerNode uint, onlyOneFullNode, turboBoost bool) (*ClusterConfig, error) {
	ccfg, err := NewClusterConfig(ts.BCfg, ts.LCfg, numNodes, numCoresPerNode, onlyOneFullNode, turboBoost)
	if err != nil {
		return nil, err
	}
	return ccfg, ccfg.StartSigmaOSCluster()
}

// Stop any running SigmaOS cluster
func (ts *Tstate) StopSigmaOSCluster() error {
	args := []string{
		"--vpc", ts.BCfg.VPC,
	}
	err := ts.LCfg.RunScriptRedirectOutputFile("./stop-sigmaos.sh", CLUSTER_INIT_LOG, args...)
	if err != nil {
		return fmt.Errorf("Err StopSigmaOSCluster: %v", err)
	}
	return nil
}

func (ts *Tstate) String() string {
	b, err := json.MarshalIndent(ts, "", "\t")
	if err != nil {
		db.DFatalf("Marshal ts: %v", err)
	}
	return string(b)
}
