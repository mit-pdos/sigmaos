package opts

import (
	"sigmaos/simms"
	"sigmaos/simms/autoscaler"
	"sigmaos/simms/lb"
)

var DefaultMicroserviceOpts simms.MicroserviceOpts = simms.MicroserviceOpts{
	NewAutoscaler:   autoscaler.NewNoOpAutoscaler,
	NewLoadBalancer: lb.NewRoundRobinLB,
}
