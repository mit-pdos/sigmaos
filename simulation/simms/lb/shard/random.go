package shard

import (
	"math/rand"

	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

// Given a set of instances, split any ready instances into shards. Return the
// shard selections, where each shard is represented by a slice of indices into
// the input instances slice.
func SelectRandomShards(instances []*simms.MicroserviceInstance, nshard int) [][]int {
	// Set up a slice to hold the shard selections
	shards := make([][]int, nshard)

	// Create slice of indices of ready instances
	nready := 0
	instanceIdxs := []int{}
	for i, r := range instances {
		if r.IsReady() {
			nready++
			instanceIdxs = append(instanceIdxs, i)
		}
	}

	if nready%nshard != 0 {
		db.DFatalf("Num ready instances not divisible by num shards: %v % %v = %v", len(instances), nshard, len(instances)%nshard != 0)
	}

	// If nshard == 1, skip the shuffle
	if nshard > 1 {
		// Shuffle the instances
		rand.Shuffle(len(instanceIdxs), func(i, j int) {
			instanceIdxs[i], instanceIdxs[j] = instanceIdxs[j], instanceIdxs[i]
		})
	}

	// Place instances into shards
	instancesPerShard := nready / nshard
	for i := 0; i < nshard; i++ {
		start := i * instancesPerShard
		end := (i + 1) * instancesPerShard
		shards[i] = instanceIdxs[start:end]
	}
	return shards
}
