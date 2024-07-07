package lb

import (
	"sigmaos/simms"
)

// Load balancer with omniscient view of microservice queue lengths, which
// distributes requests to microservice instances with the shortes queue
// lengths
type OmniscientQLenLB struct {
}

func NewOmniscientQLenLB() simms.LoadBalancer {
	return &OmniscientQLenLB{}
}

func (rr *OmniscientQLenLB) SteerRequests(reqs []*simms.Request, replicas []*simms.MicroserviceInstance) [][]*simms.Request {
	steeredReqs := make([][]*simms.Request, len(replicas))
	for i := range steeredReqs {
		steeredReqs[i] = []*simms.Request{}
	}
	// For each request
	for _, r := range reqs {
		// Get index of ready replica with smallest queue
		smallestIdx := 0
		smallestQLen := -1
		for idx := range replicas {
			if replicas[idx].IsReady() {
				// Queue length is current tick's queue length, plus number of requests to be steered to this replica in this tick
				replicaQLen := replicas[idx].GetQLen() + len(steeredReqs[idx])
				if smallestQLen == -1 || replicaQLen < smallestQLen {
					smallestQLen = replicaQLen
					smallestIdx = idx
				}
			}
		}
		// Steer request to replica with shortest queue
		steeredReqs[smallestIdx] = append(steeredReqs[smallestIdx], r)
	}
	return steeredReqs
}
