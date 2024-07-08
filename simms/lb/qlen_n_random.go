package lb

import (
	"math/rand"

	"sigmaos/simms"
	"sigmaos/simms/lb/metrics"
)

// Load balancer with omniscient view of microservice queue lengths, which
// distributes requests to microservice instances with the shortes queue
// lengths
type NRandomChoicesQLenLB struct {
	n int
}

func NewNRandomChoicesQLenLB(n int) simms.LoadBalancer {
	return &NRandomChoicesQLenLB{
		n: n,
	}
}

func (lb *NRandomChoicesQLenLB) SteerRequests(reqs []*simms.Request, instances []*simms.MicroserviceInstance) [][]*simms.Request {
	steeredReqs := make([][]*simms.Request, len(instances))
	for i := range steeredReqs {
		steeredReqs[i] = []*simms.Request{}
	}
	m := metrics.NewQLenMetric(steeredReqs, instances)
	// Create slice of indices of ready instances
	instanceIdxs := make([]int, 0, len(instances))
	for i, r := range instances {
		if r.IsReady() {
			instanceIdxs = append(instanceIdxs, i)
		}
	}
	// For each request
	for _, r := range reqs {
		// Shuffle the N instances
		rand.Shuffle(len(instanceIdxs), func(i, j int) {
			instanceIdxs[i], instanceIdxs[j] = instanceIdxs[j], instanceIdxs[i]
		})
		smallestIdx := 0
		// Sample (up to) N random choices
		for i := 0; i < lb.n && i < len(instanceIdxs); i++ {
			idx := instanceIdxs[i]
			if m.Less(idx, smallestIdx) {
				smallestIdx = idx
			}
		}
		// Steer request to instance with shortest queue
		steeredReqs[smallestIdx] = append(steeredReqs[smallestIdx], r)
	}
	return steeredReqs
}
