package opts

import (
	"sigmaos/simulation/simms"
	"sigmaos/simulation/simms/autoscaler"
	"sigmaos/simulation/simms/lb"
	lbmetrics "sigmaos/simulation/simms/lb/metrics"
	"sigmaos/simulation/simms/qmgr"
)

var DefaultMicroserviceOpts simms.MicroserviceOpts = simms.MicroserviceOpts{
	NewQMgr:               qmgr.NewBasicQMgr,
	NewAutoscaler:         autoscaler.NewNoOpAutoscaler,
	NewLoadBalancer:       lb.NewRoundRobinLB,
	NewLoadBalancerMetric: lbmetrics.NewUnsetMetric,
}
