package opts

import (
	"sigmaos/simms"
	"sigmaos/simms/autoscaler"
	"sigmaos/simms/lb"
	lbmetrics "sigmaos/simms/lb/metrics"
)

var DefaultMicroserviceOpts simms.MicroserviceOpts = simms.MicroserviceOpts{
	NewAutoscaler:         autoscaler.NewNoOpAutoscaler,
	NewLoadBalancer:       lb.NewRoundRobinLB,
	NewLoadBalancerMetric: lbmetrics.NewUnsetMetric,
}
