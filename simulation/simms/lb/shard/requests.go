package shard

import (
	"sigmaos/simulation/simms"
)

// Evenly divide requests among shards
func AssignRequestsToShardsEvenlyDistributed(reqs []*simms.Request, nshards int) []int {
	assignments := make([]int, len(reqs))
	for i := range assignments {
		// Select a shard of instances to consider
		assignments[i] = i % nshards
	}
	return assignments
}
