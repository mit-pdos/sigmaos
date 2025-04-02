package choice

import (
	"sigmaos/simulation/simms"
)

// Perform a full scan of all instances in a shard, and select the best one to
// handle a request. Return the index of the chosen instance.
func FullScan(m simms.LoadBalancerMetric, shardIdx int, shards [][]int) int {
	return RandomSubset(m, shardIdx, shards, len(shards[shardIdx]))
}
