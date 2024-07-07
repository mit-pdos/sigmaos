package lb

import (
	"sigmaos/simms"
)

// Round-robin load balancer
type RoundRobinLB struct {
}

func NewRoundRobinLB() simms.LoadBalancer {
	return &RoundRobinLB{}
}

func (rr *RoundRobinLB) SteerRequests(reqs []*simms.Request, replicas []*simms.MicroserviceInstance) [][]*simms.Request {
	steeredReqs := make([][]*simms.Request, len(replicas))
	for i := range steeredReqs {
		steeredReqs[i] = []*simms.Request{}
	}
	lastIdx := 0
	// For each request
	for _, r := range reqs {
		// Find a ready replica to process that request
		for replicaIdx := range replicas {
			idx := (lastIdx + 1 + replicaIdx) % len(replicas)
			if replicas[idx].IsReady() {
				// For the next request, start at the following replica
				lastIdx = idx
				steeredReqs[idx] = append(steeredReqs[idx], r)
				break
			}
		}
	}
	return steeredReqs
}
