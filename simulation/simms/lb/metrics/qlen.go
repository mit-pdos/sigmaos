package metrics

import (
	"sigmaos/simulation/simms"
)

type QLen struct {
	steeredReqsPerShard [][][]*simms.Request
	instances           []*simms.MicroserviceInstance
}

func NewQLenMetric(steeredReqsPerShard [][][]*simms.Request, instances []*simms.MicroserviceInstance) simms.LoadBalancerMetric {
	return &QLen{
		steeredReqsPerShard: steeredReqsPerShard,
		instances:           instances,
	}
}

func (m *QLen) Less(shard, i, j int) bool {
	// When calculating aggregate queue length, each shard only gets to know
	// about the instance's reported queue length at the beginning of the cycle,
	// as well as the requests enqueued by this shard.
	iQLen := m.instances[i].GetQLen() + len(m.steeredReqsPerShard[shard][i])
	jQLen := m.instances[j].GetQLen() + len(m.steeredReqsPerShard[shard][j])
	return iQLen < jQLen
}
