package lb

import (
	"sigmaos/simms"
)

// Load balancer with omniscient view of microservice queue lengths, which
// distributes requests to microservice instances with the shortes queue
// lengths
type OmniscientLB struct {
	newMetric simms.NewLoadBalancerMetricFn
}

func NewOmniscientLB(m simms.NewLoadBalancerMetricFn) simms.LoadBalancer {
	return &OmniscientLB{
		newMetric: m,
	}
}

func (lb *OmniscientLB) SteerRequests(reqs []*simms.Request, instances []*simms.MicroserviceInstance) [][]*simms.Request {
	steeredReqs := make([][]*simms.Request, len(instances))
	for i := range steeredReqs {
		steeredReqs[i] = []*simms.Request{}
	}
	m := lb.newMetric(steeredReqs, instances)
	// For each request
	for _, r := range reqs {
		// Get index of ready instance with smallest queue
		smallestIdx := 0
		for idx := range instances {
			if instances[idx].IsReady() {
				if m.Less(idx, smallestIdx) {
					smallestIdx = idx
				}
			}
		}
		// Steer request to instance with shortest queue
		steeredReqs[smallestIdx] = append(steeredReqs[smallestIdx], r)
	}
	return steeredReqs
}
