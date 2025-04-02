package state

import (
	"sigmaos/simulation/simms"
)

// State cache which knows all information about all replicas instantaneously.
// This cache will reshard the replicas every time it is asked to produce
// shards.
type OmniscientReshardingStateCache struct {
	t         *uint64
	instances []*simms.MicroserviceInstance
	shard     simms.LoadBalancerShardFn
	probe     simms.LoadBalancerProbeFn
	getMetric simms.LoadBalancerMetricProbeFn
}

func NewOmniscientReshardingStateCache(t *uint64, shard simms.LoadBalancerShardFn, probe simms.LoadBalancerProbeFn, getMetric simms.LoadBalancerMetricProbeFn) simms.LoadBalancerStateCache {
	return &OmniscientReshardingStateCache{
		t:         t,
		instances: nil,
		shard:     shard,
		probe:     probe,
		getMetric: getMetric,
	}
}

// Get the current queue length at an instance
func (c *OmniscientReshardingStateCache) GetStat(shard, instanceIdx int) int {
	return c.getMetric(c.instances[instanceIdx])
}

func (c *OmniscientReshardingStateCache) RunProbes(instances []*simms.MicroserviceInstance) {
	c.instances = instances
}

func (c *OmniscientReshardingStateCache) GetShards() [][]int {
	return c.shard(c.instances)
}
