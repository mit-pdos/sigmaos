package state

import (
	"sigmaos/simulation/simms"
)

// State cache which knows all information about all replicas instantaneously.
type OmniscientStateCache struct {
	instances []*simms.MicroserviceInstance
}

func NewOmniscientStateCache(instances []*simms.MicroserviceInstance) simms.LoadBalancerStateCache {
	return &OmniscientStateCache{
		instances: instances,
	}
}

// Get the current queue length at an instance
func (c *OmniscientStateCache) GetStat(shard, instanceIdx int) int {
	return c.instances[instanceIdx].GetQLen()
}
