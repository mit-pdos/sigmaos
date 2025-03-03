package opts

import (
	"sigmaos/simulation/simms"
	"sigmaos/simulation/simms/autoscaler"
	"sigmaos/simulation/simms/lb"
	lbmetrics "sigmaos/simulation/simms/lb/metrics"
	lbshard "sigmaos/simulation/simms/lb/shard"
)

var DefaultMicroserviceOpts simms.MicroserviceOpts = simms.MicroserviceOpts{
	NewQMgr:                 NewBasicQMgr,
	NewAutoscaler:           autoscaler.NewNoOpAutoscaler,
	NewLoadBalancer:         lb.NewRoundRobinLB,
	NewLoadBalancerMetric:   lbmetrics.NewUnsetMetric,
	NewLoadBalancerSharding: lbshard.SingleShard,
	KillRemovedInstances:    false,
}
