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

func (ts *Tstate) StartSigmaOSCluster(numNodes int, numCoresPerNode uint, onlyOneFullNode, turboBoost bool) (*ClusterConfig, error) {
	ccfg, err := NewClusterConfig(ts.BCfg, ts.LCfg, ts.BCfg.VPC, numNodes, numCoresPerNode, onlyOneFullNode, turboBoost)
	if err != nil {
		return nil, err
	}
	return ccfg, ccfg.StartSigmaOSCluster()
}

func (ts *Tstate) StopSigmaOSCluster() error {
	err := ts.LCfg.runScriptRedirectOutput("./stop-sigmaos.sh", CLUSTER_INIT_LOG, "--vpc", ts.BCfg.VPC)
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
