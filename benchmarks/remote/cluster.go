package remote

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type ClusterConfig struct {
	bcfg              *BenchConfig
	lcfg              *LocalFSConfig
	LeaderNodeIP      string `json:"leader_node_ip"`
	NumNodes          int    `json:"num_nodes"`
	NumCoresPerNode   uint   `json:"num_cores_per_node"`
	NumFullNodes      int    `json:"num_full_nodes"`
	NumProcqOnlyNodes int    `json:"num_besched_only_nodes"`
	TurboBoost        bool   `json:"turbo_boost"`
	UseGVisor         bool   `json:"use_gvisor"`
	// S3 prefixes to stage on every node's host before starting its kernel, so
	// that a job can read them from its local UX server (see
	// download-input-data.sh and sp.UxInputDataPath). Staging is incremental,
	// so listing a dataset that is already present costs one LIST per node.
	InputData []string `json:"input_data"`
}

func NewClusterConfig(bcfg *BenchConfig, lcfg *LocalFSConfig, numNodes int, numCoresPerNode uint, numFullNodes int, numProcqOnlyNodes int, turboBoost bool, useGVisor bool, inputData []string) (*ClusterConfig, error) {
	ccfg := &ClusterConfig{
		bcfg:              bcfg,
		lcfg:              lcfg,
		LeaderNodeIP:      sp.NOT_SET,
		NumNodes:          numNodes,
		NumCoresPerNode:   numCoresPerNode,
		NumFullNodes:      numFullNodes,
		NumProcqOnlyNodes: numProcqOnlyNodes,
		TurboBoost:        turboBoost,
		UseGVisor:         useGVisor,
		InputData:         inputData,
	}
	slIP, err := ccfg.getLeaderNodeIP()
	if err != nil {
		return nil, err
	}
	ccfg.LeaderNodeIP = slIP
	return ccfg, nil
}

func (ccfg *ClusterConfig) StartSigmaOSCluster() error {
	args := []string{
		"--vpc", ccfg.bcfg.VPC,
		"--ncores", strconv.Itoa(int(ccfg.NumCoresPerNode)),
		"--branch", ccfg.bcfg.Branch,
		"--pull", ccfg.bcfg.Tag,
		"--n", strconv.Itoa(ccfg.NumNodes),
	}
	// Append extra optional args
	if ccfg.bcfg.Overlays {
		args = append(args, "--overlays")
	}
	if ccfg.bcfg.NoNetproxy {
		args = append(args, "--nodialproxy")
	}
	if ccfg.TurboBoost {
		args = append(args, "--turbo")
	}
	if ccfg.UseGVisor {
		args = append(args, "--usegvisor")
		args = append(args, "--nodialproxy")
	}
	if ccfg.bcfg.ReloadGVisor {
		args = append(args, "--reload-gvisor-ctr")
	}
	args = append(args, "--numfullnode", strconv.Itoa(ccfg.NumFullNodes))
	args = append(args, "--numbeschednode", strconv.Itoa(ccfg.NumProcqOnlyNodes))
	if len(ccfg.InputData) > 0 {
		// One argument: the start script word-splits it to stage each dataset.
		args = append(args, "--input-data", strings.Join(ccfg.InputData, " "))
	}
	err := ccfg.lcfg.RunScriptRedirectOutputFile("./start-sigmaos.sh", CLUSTER_INIT_LOG, args...)
	if err != nil {
		return fmt.Errorf("err StopSigmaOSCluster: %v", err)
	}
	return nil
}

// Synchronously run a benchmark, according to benchCmd, on the driverVM
func (ccfg *ClusterConfig) RunBenchmark(driverVM int, benchCmd string) error {
	args := []string{
		"--vpc", ccfg.bcfg.VPC,
		"--command", benchCmd,
		"--vm", strconv.Itoa(driverVM),
	}
	if err := ccfg.lcfg.RunScriptRedirectOutputStdout("./run-benchmark.sh", args...); err != nil {
		return fmt.Errorf("err RunBenchmark: %v", err)
	}
	return nil
}

func (ccfg *ClusterConfig) CollectResults(benchName string, leaderBenchCmd, followerBenchCmd string) error {
	outDirPath := ccfg.lcfg.GetOutputDirPath(benchName)
	args := []string{
		"--parallel",
		"--vpc", ccfg.bcfg.VPC,
		"--perfdir", outDirPath,
	}
	// Write the cluster/benchmark config to the output directory
	if err := ccfg.lcfg.WriteBenchmarkConfig(outDirPath, ccfg.bcfg, ccfg, leaderBenchCmd, followerBenchCmd); err != nil {
		return fmt.Errorf("err WriteBenchmarkConfig: %v", err)
	}
	if err := ccfg.lcfg.RunScriptRedirectOutputFile("./collect-results.sh", CLUSTER_INIT_LOG, args...); err != nil {
		return fmt.Errorf("err CollectResults: %v", err)
	}
	return nil
}

// Get the IP address of the deployment's leader node
func (ccfg *ClusterConfig) getLeaderNodeIP() (string, error) {
	args := []string{
		"--vpc", ccfg.bcfg.VPC,
	}
	ip, err := ccfg.lcfg.RunScriptGetOutput("./leader-ip.sh", args...)
	if err != nil {
		return "", fmt.Errorf("err GetLeaderIP: %v", err)
	}
	return ip, nil
}

func (ccfg *ClusterConfig) String() string {
	b, err := json.MarshalIndent(ccfg, "", "\t")
	if err != nil {
		db.DFatalf("Marshal cluster config: %v", err)
	}
	return string(b)
}
