package lb

import (
	"sigmaos/simulation/simms"
)

// Load balancer with omniscient view of microservice queue lengths, which
// distributes requests to microservice instances with the shortes queue
// lengths
type CachedStateLB struct {
	qlenStateCache simms.LoadBalancerStateCache
	newMetric      simms.NewLoadBalancerMetricFn
	newShards      simms.NewLoadBalancerShardingFn
	chooseInstance simms.LoadBalancerInstanceChoiceFn
}

func NewCachedStateLB(m simms.NewLoadBalancerMetricFn, s simms.NewLoadBalancerShardingFn, c simms.LoadBalancerInstanceChoiceFn) simms.LoadBalancer {
	return &OmniscientLB{
		newMetric:      m,
		newShards:      s,
		chooseInstance: c,
	}
}

func (lb *CachedStateLB) SteerRequests(reqs []*simms.Request, instances []*simms.MicroserviceInstance) [][]*simms.Request {
	instanceShards := lb.newShards(instances)
	steeredReqsPerShard := make([][][]*simms.Request, len(instanceShards))
	for i := range instanceShards {
		steeredReqsPerShard[i] = make([][]*simms.Request, len(instances))
		for j := range instances {
			steeredReqsPerShard[i][j] = []*simms.Request{}
		}
	}
	m := lb.newMetric(lb.qlenStateCache, steeredReqsPerShard, instances)
	instanceShardIdx := 0
	// For each request
	for _, r := range reqs {
		// Select a shard of instances to consider
		shardIdx := instanceShardIdx % len(instanceShards)
		// Choose the instance in this shard which is the best fit to handle the
		// request
		bestFitIdx := lb.chooseInstance(m, shardIdx, instanceShards)
		// Steer request to instance with best fit
		steeredReqsPerShard[shardIdx][bestFitIdx] = append(steeredReqsPerShard[shardIdx][bestFitIdx], r)
		// Move on to the next instance shard
		instanceShardIdx++
	}
	return mergeSteeredReqsPerShard(len(instanceShards), steeredReqsPerShard)
}
