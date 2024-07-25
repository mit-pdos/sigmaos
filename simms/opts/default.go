package opts

import (
	"sigmaos/simms"
	"sigmaos/simms/autoscaler"
	"sigmaos/simms/lb"
	lbmetrics "sigmaos/simms/lb/metrics"
	"sigmaos/simms/qmgr"
)

var DefaultMicroserviceOpts simms.MicroserviceOpts = simms.MicroserviceOpts{
	NewQMgr:               qmgr.NewBasicQMgr,
	NewAutoscaler:         autoscaler.NewNoOpAutoscaler,
	NewLoadBalancer:       lb.NewRoundRobinLB,
	NewLoadBalancerMetric: lbmetrics.NewUnsetMetric,
}
