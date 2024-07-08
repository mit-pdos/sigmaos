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

func (lb *OmniscientQLenLB) SteerRequests(reqs []*simms.Request, instances []*simms.MicroserviceInstance) [][]*simms.Request {
	steeredReqs := make([][]*simms.Request, len(instances))
	for i := range steeredReqs {
		steeredReqs[i] = []*simms.Request{}
	}
	// For each request
	for _, r := range reqs {
		// Get index of ready instance with smallest queue
		smallestIdx := 0
		smallestQLen := -1
		for idx := range instances {
			if instances[idx].IsReady() {
				// Queue length is current tick's queue length, plus number of requests to be steered to this instance in this tick
				instanceQLen := instances[idx].GetQLen() + len(steeredReqs[idx])
				if smallestQLen == -1 || instanceQLen < smallestQLen {
					smallestQLen = instanceQLen
					smallestIdx = idx
				}
			}
		}
		// Steer request to instance with shortest queue
		steeredReqs[smallestIdx] = append(steeredReqs[smallestIdx], r)
	}
	return steeredReqs
}
