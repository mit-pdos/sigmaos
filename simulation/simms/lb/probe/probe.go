package probe

import (
	"math/rand"

	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

// Probe all instances in a shard
func ProbeAll(m simms.LoadBalancerMetricProbeFn, instances []*simms.MicroserviceInstance, shards [][]int) [][]*simms.LoadBalancerProbeResult {
	probeResults := make([][]*simms.LoadBalancerProbeResult, len(shards))
	for shardIdx := range probeResults {
		shard := shards[shardIdx]
		probeResults[shardIdx] = make([]*simms.LoadBalancerProbeResult, len(shard))
		for i := range shard {
			instanceIdx := shard[i]
			probeResults[shardIdx][i] = simms.NewLoadBalancerProbeResult(instanceIdx, m(instances[instanceIdx]))
		}
	}
	return probeResults
}

// Probe all instances in a shard, plus N random instances not in the shard
func ProbeAllPlusNNew(nNew int, m simms.LoadBalancerMetricProbeFn, instances []*simms.MicroserviceInstance, shards [][]int) [][]*simms.LoadBalancerProbeResult {
	// TODO: dedup this code with sharding
	// Get slice of ready instances
	nready := 0
	readyInstances := []int{}
	for i, r := range instances {
		if r.IsReady() {
			nready++
			readyInstances = append(readyInstances, i)
		}
	}
	probeResults := make([][]*simms.LoadBalancerProbeResult, len(shards))
	for shardIdx := range probeResults {
		shard := shards[shardIdx]
		// Store the probe results for this shard
		probeResults[shardIdx] = make([]*simms.LoadBalancerProbeResult, 0, len(shard)+nNew)
		// Record which instances are in this shard
		instancesInShard := make(map[int]bool)
		for i := range shard {
			instanceIdx := shard[i]
			instancesInShard[instanceIdx] = true
			probeResults[shardIdx] = append(probeResults[shardIdx], simms.NewLoadBalancerProbeResult(instanceIdx, m(instances[instanceIdx])))
		}
		// Shuffle the instances
		rand.Shuffle(len(readyInstances), func(i, j int) {
			readyInstances[i], readyInstances[j] = readyInstances[j], readyInstances[i]
		})
		// Add N additional probes to the results
		for i := len(shard); i < len(shard)+nNew && i < len(readyInstances); i++ {
			instanceIdx := readyInstances[i]
			// If the instance is not already in the shard, probe it
			if ok := instancesInShard[instanceIdx]; !ok {
				probeResults[shardIdx] = append(probeResults[shardIdx], simms.NewLoadBalancerProbeResult(instanceIdx, m(instances[instanceIdx])))
				db.DPrintf(db.SIM_LB_PROBE, "shard %v additional probe %v", shardIdx, instanceIdx)
			}
		}
	}
	return probeResults
}

func ProbeNOldPlusNNew(nOld int, nNew int, m simms.LoadBalancerMetricProbeFn, instances []*simms.MicroserviceInstance, shards [][]int) [][]*simms.LoadBalancerProbeResult {
	// TODO: dedup this code with sharding
	// Get slice of ready instances
	nready := 0
	readyInstances := []int{}
	for i, r := range instances {
		if r.IsReady() {
			nready++
			readyInstances = append(readyInstances, i)
		}
	}
	probeResults := make([][]*simms.LoadBalancerProbeResult, len(shards))
	for shardIdx := range probeResults {
		shard := shards[shardIdx]
		// Store the probe results for this shard
		probeResults[shardIdx] = make([]*simms.LoadBalancerProbeResult, 0, nOld+nNew)
		// Record which instances are in this shard
		instancesInShard := make(map[int]bool)
		for i := range shard {
			instanceIdx := shard[i]
			instancesInShard[instanceIdx] = true
		}
		// Shuffle the shard instances
		shuffledShard := make([]int, len(shard))
		copy(shuffledShard, shard)
		rand.Shuffle(len(shuffledShard), func(i, j int) {
			shuffledShard[i], shuffledShard[j] = shuffledShard[j], shuffledShard[i]
		})
		// Probe a random nOld instances in the shard
		for i := 0; i < nOld; i++ {
			instanceIdx := shuffledShard[i]
			probeResults[shardIdx] = append(probeResults[shardIdx], simms.NewLoadBalancerProbeResult(instanceIdx, m(instances[instanceIdx])))
		}
		// Shuffle the full set of ready instances
		rand.Shuffle(len(readyInstances), func(i, j int) {
			readyInstances[i], readyInstances[j] = readyInstances[j], readyInstances[i]
		})
		// Add N additional probes to the results
		for i := len(shard); i < len(shard)+nNew; i++ {
			instanceIdx := readyInstances[i]
			// If the instance is not already in the shard, probe it
			if ok := instancesInShard[instanceIdx]; !ok {
				probeResults[shardIdx] = append(probeResults[shardIdx], simms.NewLoadBalancerProbeResult(instanceIdx, m(instances[instanceIdx])))
			}
		}
	}
	return probeResults
}
