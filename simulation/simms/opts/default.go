package opts

import (
	"sigmaos/simulation/simms"
	"sigmaos/simulation/simms/autoscaler"
	"sigmaos/simulation/simms/lb"
	lbchoice "sigmaos/simulation/simms/lb/choice"
	lbmetrics "sigmaos/simulation/simms/lb/metrics"
	lbprobe "sigmaos/simulation/simms/lb/probe"
	lbshard "sigmaos/simulation/simms/lb/shard"
	lbstate "sigmaos/simulation/simms/lb/state"
)

var DefaultMicroserviceOpts simms.MicroserviceOpts = simms.MicroserviceOpts{
	NewQMgr:                    NewBasicQMgr,
	NewAutoscaler:              autoscaler.NewNoOpAutoscaler,
	NewLoadBalancer:            lb.NewRoundRobinLB,
	NewLoadBalancerMetric:      lbmetrics.NewUnsetMetric,
	LoadBalancerMetricProbe:    lbmetrics.GetQLen,
	NewLoadBalancerStateCache:  lbstate.NewOmniscientReshardingStateCache,
	LoadBalancerShard:          lbshard.SingleShard,
	LoadBalancerProbe:          lbprobe.ProbeAll,
	LoadBalancerInstanceChoice: lbchoice.FullScan,
	KillRemovedInstances:       false,
}
