package shard

import (
	"math/rand"

	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

// Given a set of instances, split any ready instances into non-overlapping
// shards. Return the shard selections, where each shard is represented by a
// slice of indices into the input instances slice.
func SelectNonOverlappingRandomShards(instances []*simms.MicroserviceInstance, nshard int) [][]int {
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

	// Sanity check
	for _, s := range shards {
		for _, idx := range s {
			if !instances[idx].IsReady() {
				db.DFatalf("Unready instance included in shard selection: idx %v", idx)
			}
		}
	}
	return shards
}

// Given a set of instances, split any ready instances into
// (potentially-overlapping) shards. Return the shard selections, where each
// shard is represented by a slice of indices into the input instances slice.
func SelectOverlappingRandomShards(instances []*simms.MicroserviceInstance, nshard int, nInstancesPerShard int) [][]int {
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

	// Place instances into shards
	for i := range shards {
		// Shuffle the instances
		rand.Shuffle(len(instanceIdxs), func(i, j int) {
			instanceIdxs[i], instanceIdxs[j] = instanceIdxs[j], instanceIdxs[i]
		})
		// Select the first N instances to be part of this shard
		shards[i] = make([]int, nInstancesPerShard)
		copy(shards[i], instanceIdxs[:nInstancesPerShard])
	}

	// Sanity check
	for _, s := range shards {
		for _, idx := range s {
			if !instances[idx].IsReady() {
				db.DFatalf("Unready instance included in shard selection: idx %v", idx)
			}
		}
	}
	return shards
}

// Given a set of instances, split any ready instances into
// (potentially-overlapping) shards. Shards are initially randomized, but split
// deterministically. Return the shard selections, where each shard is
// represented by a slice of indices into the input instances slice.
//
// Taken from Google's deterministic subsetting algorithm: https://sre.google/sre-book/load-balancing-datacenter/
func SelectDeterministicSubsettingShards(instances []*simms.MicroserviceInstance, nshard int, nInstancesPerShard int) [][]int {
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
	subsetSize := nInstancesPerShard
	subsetCnt := len(instanceIdxs) / subsetSize
	// Place instances into shards
	for shardIdx := 0; shardIdx < len(shards); shardIdx++ {
		// Shuffle once we have fully subsetted this random assignment of instances
		if shardIdx%subsetCnt == 0 {
			// Shuffle the instances
			rand.Shuffle(len(instanceIdxs), func(i, j int) {
				instanceIdxs[i], instanceIdxs[j] = instanceIdxs[j], instanceIdxs[i]
			})
		}
		// Select the first N instances to be part of this shard
		shards[shardIdx] = make([]int, subsetSize)
		subsetID := shardIdx % subsetCnt
		start := subsetID * subsetSize
		copy(shards[shardIdx], instanceIdxs[start:start+subsetSize])
	}
	// Sanity check
	for _, s := range shards {
		for _, idx := range s {
			if !instances[idx].IsReady() {
				db.DFatalf("Unready instance included in shard selection: idx %v", idx)
			}
		}
	}
	return shards
}
