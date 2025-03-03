package lb

import (
	"sigmaos/simulation/simms"
)

// Load balancer with omniscient view of microservice queue lengths, which
// distributes requests to microservice instances with the shortes queue
// lengths
type OmniscientLB struct {
	newMetric      simms.NewLoadBalancerMetricFn
	newShards      simms.NewLoadBalancerShardingFn
	chooseInstance simms.LoadBalancerInstanceChoiceFn
}

func NewOmniscientLB(m simms.NewLoadBalancerMetricFn, s simms.NewLoadBalancerShardingFn, c simms.LoadBalancerInstanceChoiceFn) simms.LoadBalancer {
	return &OmniscientLB{
		newMetric:      m,
		newShards:      s,
		chooseInstance: c,
	}
}

func (lb *OmniscientLB) SteerRequests(reqs []*simms.Request, instances []*simms.MicroserviceInstance) [][]*simms.Request {
	steeredReqs := make([][]*simms.Request, len(instances))
	for i := range steeredReqs {
		steeredReqs[i] = []*simms.Request{}
	}
	m := lb.newMetric(steeredReqs, instances)
	instanceShards := lb.newShards(instances)
	instanceShardIdx := 0
	// For each request
	for _, r := range reqs {
		// Select a shard of instances to consider
		shardIdx := instanceShardIdx % len(instanceShards)
		// Choose the instance in this shard which is the best fit to handle the
		// request
		bestFitIdx := lb.chooseInstance(m, shardIdx, instanceShards)
		// Steer request to instance with shortest queue
		steeredReqs[bestFitIdx] = append(steeredReqs[bestFitIdx], r)
		// Move on to the next instance shard
		instanceShardIdx++
	}
	return steeredReqs
}
