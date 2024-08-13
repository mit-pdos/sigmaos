package remote

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
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

func init() {
	flag.StringVar(&platformArg, "platform", sp.NOT_SET, "Platform on which to run. Currently, only [aws|cloudlab] are supported")
	flag.StringVar(&vpcArg, "vpc", sp.NOT_SET, "VPC in which to run. Need not be specified for Cloudlab.")
	flag.StringVar(&tagArg, "tag", sp.NOT_SET, "Build tag with which to run.")
	flag.StringVar(&branchArg, "branch", "master", "Branch on which to run.")
	flag.StringVar(&versionArg, "version", sp.NOT_SET, "Output version string.")
	flag.BoolVar(&noNetproxyArg, "nonetproxy", false, "Disable use of proxy for network dialing/listening.")
	flag.BoolVar(&overlaysArg, "overlays", false, "Run with Docker swarm overlays enabled.")
	flag.BoolVar(&parallelArg, "parallelize", false, "Run commands in parallel to speed up, e.g., cluster shutdown.")
}

func TestCompile(t *testing.T) {
}

func TestStartStop(t *testing.T) {
	// Cluster configuration parameters
	const (
		numNodes        int  = 4
		numCoresPerNode uint = 4
		onlyOneFullNode bool = false
		turboBoost      bool = false
	)
	ts, err := NewTstate(t)
	if !assert.Nil(ts.t, err, "Creating test state: %v", err) {
		return
	}
	if err := ts.StopSigmaOSCluster(); !assert.Nil(ts.t, err, "Stop cluster: %v", err) {
		return
	}
	ccfg, err := ts.StartSigmaOSCluster(4, 4, false, false)
	db.DPrintf(db.ALWAYS, "Running remote tests:\n%v\nCluster config:\n%v", ts, ccfg)
	if !assert.Nil(ts.t, err, "Start cluster: %v", err) {
		return
	}
}
