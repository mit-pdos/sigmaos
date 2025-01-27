package metrics

import (
	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

type Unset struct{}

func NewUnsetMetric(steeredReqs [][]*simms.Request, instances []*simms.MicroserviceInstance) simms.LoadBalancerMetric {
	return &Unset{}
}

func (m *Unset) Less(i, j int) bool {
	db.DFatalf("Load balancer metrics unset")
	return false
}
