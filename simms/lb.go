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
	for i, r := range reqs {
		idx := i % len(steeredReqs)
		steeredReqs[idx] = append(steeredReqs[idx], r)
	}
	return steeredReqs
}
