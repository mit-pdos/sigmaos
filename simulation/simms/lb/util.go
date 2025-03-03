package lb

import (
	"sigmaos/simulation/simms"
)

func mergeSteeredReqsPerShard(ninstances int, steeredReqsPerShard [][][]*simms.Request) [][]*simms.Request {
	steeredReqs := make([][]*simms.Request, ninstances)
	for i := range steeredReqs {
		steeredReqs[i] = []*simms.Request{}
	}
	for _, shardSteered := range steeredReqsPerShard {
		for i := range steeredReqs {
			steeredReqs[i] = append(steeredReqs[i], shardSteered[i]...)
		}
	}
	return steeredReqs
}
