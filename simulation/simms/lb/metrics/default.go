package metrics

import (
	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

type Unset struct{}

func NewUnsetMetric(lbStateCache simms.LoadBalancerStateCache, steeredReqsPerShard [][][]*simms.Request) simms.LoadBalancerMetric {
	return &Unset{}
}

func (m *Unset) Less(shard, i, j int) bool {
	db.DFatalf("Load balancer metrics unset")
	return false
}
