package remote

import (
	"encoding/json"
	"fmt"
	"strconv"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type ClusterConfig struct {
	bcfg            *BenchConfig
	lcfg            *LocalFSConfig
	LeaderNodeIP    string `json:"leader_node_ip"`
	NumNodes        int    `json:"num_nodes"`
	NumCoresPerNode uint   `json:"num_cores_per_node"`
	OnlyOneFullNode bool   `json:"only_one_full_node"`
	TurboBoost      bool   `json:"turbo_boost"`
}

func NewClusterConfig(bcfg *BenchConfig, lcfg *LocalFSConfig, numNodes int, numCoresPerNode uint, onlyOneFullNode, turboBoost bool) (*ClusterConfig, error) {
	ccfg := &ClusterConfig{
		bcfg:            bcfg,
		lcfg:            lcfg,
		LeaderNodeIP:    sp.NOT_SET,
		NumNodes:        numNodes,
		NumCoresPerNode: numCoresPerNode,
		OnlyOneFullNode: onlyOneFullNode,
		TurboBoost:      turboBoost,
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
		args = append(args, "--nonetproxy")
	}
	if ccfg.TurboBoost {
		args = append(args, "--turbo")
	}
	if ccfg.OnlyOneFullNode {
		args = append(args, "--nodetype", "minnode")
	}
	err := ccfg.lcfg.RunScriptRedirectOutputFile("./start-sigmaos.sh", CLUSTER_INIT_LOG, args...)
	if err != nil {
		return fmt.Errorf("Err StopSigmaOSCluster: %v", err)
	}
	return nil
}

// Synchronously run a benchmark, according to benchCmd, on the driverVM
func (ccfg *ClusterConfig) RunBenchmark(benchCmd string, driverVM int) error {
	args := []string{
		"--vpc", ccfg.bcfg.VPC,
		"--command", benchCmd,
		"--vm", strconv.Itoa(driverVM),
	}
	if err := ccfg.lcfg.RunScriptRedirectOutputStdout("./run-benchmark.sh", args...); err != nil {
		return fmt.Errorf("Err RunBenchmark: %v", err)
	}
	return nil
}

func (ccfg *ClusterConfig) CollectResults(benchName string) error {
	outDirPath := ccfg.lcfg.GetOutputDirPath(benchName)
	args := []string{
		"--vpc", ccfg.bcfg.VPC,
		"--perfdir", outDirPath,
	}
	err := ccfg.lcfg.RunScriptRedirectOutputFile("./collect-results.sh", CLUSTER_INIT_LOG, args...)
	if err != nil {
		return fmt.Errorf("Err CollectResults: %v", err)
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
		return "", fmt.Errorf("Err GetLeaderIP: %v", err)
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
