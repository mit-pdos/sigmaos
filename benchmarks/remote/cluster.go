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
	VPC             string `json:"vpc"`
	LeaderNodeIP    string `json:"leader_node_ip"`
	NumNodes        int    `json:"num_nodes"`
	NumCoresPerNode uint   `json:"num_cores_per_node"`
	OnlyOneFullNode bool   `json:"only_one_full_node"`
	TurboBoost      bool   `json:"turbo_boost"`
}

func NewClusterConfig(bcfg *BenchConfig, lcfg *LocalFSConfig, vpc string, numNodes int, numCoresPerNode uint, onlyOneFullNode, turboBoost bool) (*ClusterConfig, error) {
	ccfg := &ClusterConfig{
		bcfg:            bcfg,
		lcfg:            lcfg,
		VPC:             vpc,
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
	err := ccfg.lcfg.runScriptRedirectOutput("./start-sigmaos.sh", CLUSTER_INIT_LOG, args...)
	if err != nil {
		return fmt.Errorf("Err StopSigmaOSCluster: %v", err)
	}
	return nil
}

// Get the IP address of the deployment's leader node
func (ccfg *ClusterConfig) getLeaderNodeIP() (string, error) {
	ip, err := ccfg.lcfg.runScriptGetOutput("./leader-ip.sh", "--vpc", ccfg.VPC)
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
