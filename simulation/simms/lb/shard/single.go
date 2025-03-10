package shard

import (
	"sigmaos/simulation/simms"
)

// Group all instances into a single shard
func SingleShard(instances []*simms.MicroserviceInstance) [][]int {
	return SelectNonOverlappingRandomShards(instances, 1)
}
