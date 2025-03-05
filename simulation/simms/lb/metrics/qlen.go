package metrics

import (
	"sigmaos/simulation/simms"
)

type QLen struct {
	lbStateCache        simms.LoadBalancerStateCache
	steeredReqsPerShard [][][]*simms.Request
}

// Metric based on cached queue length. If no queue length state is cached for
// a given instance, return random choice.
func NewQLenMetric(lbStateCache simms.LoadBalancerStateCache, steeredReqsPerShard [][][]*simms.Request) simms.LoadBalancerMetric {
	return &QLen{
		lbStateCache:        lbStateCache,
		steeredReqsPerShard: steeredReqsPerShard,
	}
}

func (m *QLen) Less(shard, i, j int) bool {
	// When calculating aggregate queue length, each shard only gets to know
	// about the instance's reported queue length at the beginning of the cycle,
	// as well as the requests enqueued by this shard.
	iQLen := m.lbStateCache.GetStat(shard, i) + len(m.steeredReqsPerShard[shard][i])
	jQLen := m.lbStateCache.GetStat(shard, j) + len(m.steeredReqsPerShard[shard][j])
	return iQLen < jQLen
}
