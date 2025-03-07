package lb

import (
	"sigmaos/simulation/simms"
)

// Load balancer with omniscient view of microservice queue lengths, which
// distributes requests to microservice instances with the shortes queue
// lengths
type OmniscientLB struct {
	t              *uint64
	stateCache     simms.LoadBalancerStateCache
	newMetric      simms.NewLoadBalancerMetricFn
	chooseInstance simms.LoadBalancerInstanceChoiceFn
	assignReqs     simms.AssignRequestsToLoadBalancerShardsFn
}

func NewOmniscientLB(t *uint64, stateCache simms.LoadBalancerStateCache, m simms.NewLoadBalancerMetricFn, c simms.LoadBalancerInstanceChoiceFn, assignReqs simms.AssignRequestsToLoadBalancerShardsFn) simms.LoadBalancer {
	return &OmniscientLB{
		t:              t,
		stateCache:     stateCache,
		newMetric:      m,
		chooseInstance: c,
		assignReqs:     assignReqs,
	}
}

func (lb *OmniscientLB) SteerRequests(reqs []*simms.Request, instances []*simms.MicroserviceInstance) [][]*simms.Request {
	// Probe instances, and adjust shards as necessary
	lb.stateCache.RunProbes(instances)
	// Get the assignment of instances to shards
	instanceShards := lb.stateCache.GetShards()
	steeredReqsPerShard := make([][][]*simms.Request, len(instanceShards))
	for i := range instanceShards {
		steeredReqsPerShard[i] = make([][]*simms.Request, len(instances))
		for j := range instances {
			steeredReqsPerShard[i][j] = []*simms.Request{}
		}
	}
	m := lb.newMetric(lb.stateCache, steeredReqsPerShard)
	reqShardAssignments := lb.assignReqs(reqs, len(instanceShards))
	instanceShardIdx := 0
	// For each request
	for reqIdx, r := range reqs {
		// Select a shard of instances to consider
		shardIdx := reqShardAssignments[reqIdx]
		// Choose the instance in this shard which is the best fit to handle the
		// request
		bestFitIdx := lb.chooseInstance(m, shardIdx, instanceShards)
		// Steer request to instance with best fit
		steeredReqsPerShard[shardIdx][bestFitIdx] = append(steeredReqsPerShard[shardIdx][bestFitIdx], r)
		// Move on to the next instance shard
		instanceShardIdx++
	}
	return mergeSteeredReqsPerShard(len(instances), steeredReqsPerShard)
}
