package lb

import (
	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

// Load balancer with omniscient view of microservice queue lengths, which
// distributes requests to microservice instances with the shortes queue
// lengths
type OmniscientLB struct {
	newMetric  simms.NewLoadBalancerMetricFn
	shardingFn simms.NewLoadBalancerShardingFn
}

func NewOmniscientLB(m simms.NewLoadBalancerMetricFn, s simms.NewLoadBalancerShardingFn) simms.LoadBalancer {
	return &OmniscientLB{
		newMetric:  m,
		shardingFn: s,
	}
}

func (lb *OmniscientLB) SteerRequests(reqs []*simms.Request, instances []*simms.MicroserviceInstance) [][]*simms.Request {
	steeredReqs := make([][]*simms.Request, len(instances))
	for i := range steeredReqs {
		steeredReqs[i] = []*simms.Request{}
	}
	m := lb.newMetric(steeredReqs, instances)
	instanceShards := lb.shardingFn(instances)
	instanceShardIdx := 0
	// For each request
	for _, r := range reqs {
		// Get index of ready instance with smallest queue
		smallestIdx := 0
		// Select a shard of instances to consider
		instanceShard := instanceShards[instanceShardIdx%len(instanceShards)]
		// Iterate each instance in the shard
		for _, idx := range instanceShard {
			// Sanity check
			if !instances[idx].IsReady() {
				db.DFatalf("Unready instance included in shard selection: idx %v", idx)
			}
			// Check if this shard's instance is more suitable
			if m.Less(idx, smallestIdx) {
				smallestIdx = idx
			}
		}
		// Steer request to instance with shortest queue
		steeredReqs[smallestIdx] = append(steeredReqs[smallestIdx], r)
		// Move on to the next instance shard
		instanceShardIdx++
	}
	return steeredReqs
}
