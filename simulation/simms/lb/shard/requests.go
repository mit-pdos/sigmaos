package shard

import (
	"math"
	"math/rand"

	"sigmaos/simulation/simms"
)

// Evenly divide requests among shards
func AssignRequestsToShardsUniformly(reqs []*simms.Request, nshards int) []int {
	assignments := make([]int, len(reqs))
	for i := range assignments {
		// Select a shard of instances to consider
		assignments[i] = i % nshards
	}
	return assignments
}

// Divide requests among shards according to a gaussian distribution
func AssignRequestsToShardsGaussian(reqs []*simms.Request, nshards int) []int {
	// Permute the shard indexes, so that adjacent shards don't happen to get
	// "similar" numbers of requests
	shardPermutation := rand.Perm(nshards)
	assignments := make([]int, len(reqs))
	for i := range assignments {
		// Pull a Gaussian/normally distributed number, with std such that 99.7% of
		// the pulls will be in the range [0, nshards)
		r := math.Abs(rand.NormFloat64())*float64(nshards)/3.0 + float64(nshards)/2.0
		// In the rare case that r > nshards, truncate
		shardIdx := int(math.Round(r)) % nshards
		// Select a shard of instances to consider
		assignments[i] = shardPermutation[shardIdx]
	}
	return assignments
}
