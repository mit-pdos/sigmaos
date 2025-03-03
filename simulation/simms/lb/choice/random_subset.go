package choice

import (
	"math/rand"

	"sigmaos/simulation/simms"
)

// Perform a scan of a random subset of instances in a shard, and select the
// best one to handle a request. Return the index of the chosen instance.
func RandomSubset(m simms.LoadBalancerMetric, shardIdx int, shards [][]int, nchoices int) int {
	instanceShard := shards[shardIdx]
	instanceIdxs := make([]int, len(instanceShard))
	copy(instanceIdxs, instanceShard)
	// Skip the shuffle if all instances will be considered anyway
	if nchoices < len(instanceShard) {
		// Shuffle the N instances
		rand.Shuffle(len(instanceIdxs), func(i, j int) {
			instanceIdxs[i], instanceIdxs[j] = instanceIdxs[j], instanceIdxs[i]
		})
	}
	// Get index of ready instance with smallest queue
	smallestIdx := 0
	// Sample (up to) N random choices
	for i := 0; i < nchoices && i < len(instanceIdxs); i++ {
		idx := instanceIdxs[i]
		if m.Less(idx, smallestIdx) {
			smallestIdx = idx
		}
	}
	return smallestIdx
}
