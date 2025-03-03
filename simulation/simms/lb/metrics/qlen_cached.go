package metrics

import (
	"sigmaos/simulation/simms"
)

type QLenCached struct {
	lbStateCache        simms.LoadBalancerStateCache
	steeredReqsPerShard [][][]*simms.Request
	instances           []*simms.MicroserviceInstance
}

// Metric based on cached queue length. If no queue length state is cached for
// a given instance, return random choice.
func NewQLenCachedMetric(lbStateCache simms.LoadBalancerStateCache, steeredReqsPerShard [][][]*simms.Request, instances []*simms.MicroserviceInstance) simms.LoadBalancerMetric {
	return &QLenCached{
		lbStateCache:        lbStateCache,
		steeredReqsPerShard: steeredReqsPerShard,
		instances:           instances,
	}
}

func (m *QLenCached) Less(shard, i, j int) bool {
	// When calculating aggregate queue length, each shard only gets to know
	// about the instance's reported queue length at the beginning of the cycle,
	// as well as the requests enqueued by this shard.
	iQLen := m.instances[i].GetQLen() + len(m.steeredReqsPerShard[shard][i])
	jQLen := m.instances[j].GetQLen() + len(m.steeredReqsPerShard[shard][j])
	return iQLen < jQLen
}
