package remote

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
)

type GetBenchCmdFn func(*BenchConfig, *ClusterConfig) string
type K8sAppMgmtFn func(*BenchConfig, *LocalFSConfig) error

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

// Run a benchmark with multiple parallel clients:
// 1. Create dirs to hold benchmark output
// 2. Stop running SigmaOS clusters
// 3. Start a SigmaOS cluster
// 4. Start benchmark clients, beginning with the leader and subsequently the
// followers, in parallel with a short delay
// 5. Wait for all clients to complete
// 6. Collect benchmark results
// 7. Stop the SigmaOS cluster
//
// If any of the above steps results in an error, bail out early.
func (ts *Tstate) RunParallelClientBenchmark(benchName string, driverVMs []int, getLeaderClientBenchCmd GetBenchCmdFn, getFollowerClientBenchCmd GetBenchCmdFn, startK8sApp K8sAppMgmtFn, stopK8sApp K8sAppMgmtFn, clientDelay time.Duration, numNodes int, numCoresPerNode uint, numFullNodes int, numProcqOnlyNodes int, turboBoost bool) bool {
	db.DPrintf(db.ALWAYS, "========== Run benchmark %v ==========", benchName)
	// Set up the benchmark, and bail out if the benchmark already ran
	if alreadyRan, err := ts.PrepareToRunBenchmark(benchName); !assert.Nil(ts.t, err, "Prepare benchmark: %v", err) {
		return false
	} else if alreadyRan {
		db.DPrintf(db.ALWAYS, "========== Skipping %v (already ran) ==========", benchName)
		return false
	}
	// First, stop any previously running cluster
	if err := ts.StopSigmaOSCluster(); !assert.Nil(ts.t, err, "Stop cluster: %v", err) {
		return false
	}
	// Start a SigmaOS cluster
	ccfg, err := ts.StartSigmaOSCluster(numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
	db.DPrintf(db.ALWAYS, "\nCluster config:\n%v", ccfg)
	if !assert.Nil(ts.t, err, "Start SigmaOS cluster: %v", err) {
		return false
	}
	// If running the k8s version of the benchmark, start the k8s app
	if ts.BCfg.K8s {
		err := startK8sApp(ts.BCfg, ts.LCfg)
		if !assert.Nil(ts.t, err, "Start k8s app: %v", err) {
			return false
		}
		defer func() {
			err := stopK8sApp(ts.BCfg, ts.LCfg)
			if !assert.Nil(ts.t, err, "Stop k8s app: %v", err) {
				return
			}
		}()
	}
	// Optionally skip shutting down the cluster after the benchmark completes
	// (useful for debugging)
	if !ts.BCfg.NoShutdown {
		defer func() {
			// Stop the SigmaOS cluster once the benchmark is over
			err := ts.StopSigmaOSCluster()
			assert.Nil(ts.t, err, "Stop SigmaOS cluster: %v", err)
		}()
	}
	leaderBenchCmd := getLeaderClientBenchCmd(ts.BCfg, ccfg)
	followerBenchCmd := "no-cmd"
	if getFollowerClientBenchCmd != nil {
		followerBenchCmd = getFollowerClientBenchCmd(ts.BCfg, ccfg)
	}
	done := make(chan error)
	for i := 0; i < len(driverVMs); i++ {
		// Select the driver VM on which to run this client
		driverVM := driverVMs[i]
		// Select the command string to run the benchmark
		var cmd string
		if i == 0 {
			cmd = leaderBenchCmd
		} else {
			cmd = followerBenchCmd
			db.DPrintf(db.ALWAYS, "\t----- Starting additional benchmark client %v -----", i)
		}
		// Start the benchmark client in a different goroutine
		go func(i int, driverVM int, cmd string) {
			err = ccfg.RunBenchmark(driverVM, cmd)
			assert.Nil(ts.t, err, "Run benchmark client %v: %v", err)
			// Mark self as done
			done <- err
		}(i, driverVM, cmd)
		// Sleep for a short delay before starting the next client
		time.Sleep(clientDelay)
	}
	// Wait for clients to finish
	for i := 0; i < len(driverVMs); i++ {
		<-done
	}
	// Collect the benchmark results
	if err := ccfg.CollectResults(benchName, leaderBenchCmd, followerBenchCmd); !assert.Nil(ts.t, err, "CollectResults: %v", err) {
		return true
	}
	return true
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
func (ts *Tstate) RunStandardBenchmark(benchName string, driverVM int, getBenchCmd GetBenchCmdFn, numNodes int, numCoresPerNode uint, numFullNodes int, numProcqOnlyNodes int, turboBoost bool) {
	ts.RunParallelClientBenchmark(benchName, []int{driverVM}, getBenchCmd, nil, nil, nil, 0*time.Second, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
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
func (ts *Tstate) StartSigmaOSCluster(numNodes int, numCoresPerNode uint, numFullNodes int, numProcqOnlyNodes int, turboBoost bool) (*ClusterConfig, error) {
	ccfg, err := NewClusterConfig(ts.BCfg, ts.LCfg, numNodes, numCoresPerNode, numFullNodes, numProcqOnlyNodes, turboBoost)
	if err != nil {
		return nil, err
	}
	return ccfg, ccfg.StartSigmaOSCluster()
}

// Stop any running SigmaOS cluster
func (ts *Tstate) StopSigmaOSCluster() error {
	args := []string{
		"--parallel",
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
