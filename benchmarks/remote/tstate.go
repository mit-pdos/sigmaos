package remote

import (
	"encoding/json"
	"fmt"
	"testing"

	db "sigmaos/debug"
)

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
