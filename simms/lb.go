package simms

type LoadBalancer interface {
	SteerRequests([]*Request, []*MicroserviceInstance) [][]*Request
}

// Round-robin load balancer
type RoundRobinLB struct {
}

func NewRoundRobinLB() *RoundRobinLB {
	return &RoundRobinLB{}
}

func (rr *RoundRobinLB) SteerRequests(reqs []*Request, replicas []*MicroserviceInstance) [][]*Request {
	steeredReqs := make([][]*Request, len(replicas))
	for i := range steeredReqs {
		steeredReqs[i] = []*Request{}
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
