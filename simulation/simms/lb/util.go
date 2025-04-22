package lb

import (
	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

func mergeSteeredReqsPerShard(ninstances int, steeredReqsPerShard [][][]*simms.Request) [][]*simms.Request {
	steeredReqs := make([][]*simms.Request, ninstances)
	for i := range steeredReqs {
		steeredReqs[i] = []*simms.Request{}
	}
	for shardIdx, shardSteered := range steeredReqsPerShard {
		if db.WillBePrinted(db.SIM_LB_SHARD) {
			total := 0
			steeredReqsCnt := make([]int, len(shardSteered))
			for i, r := range shardSteered {
				steeredReqsCnt[i] = len(r)
				total += len(r)
			}
			db.DPrintf(db.SIM_LB_SHARD, "Shard %v steered [total=%v]:\n%v", shardIdx, total, steeredReqsCnt)
		}
		for i := range steeredReqs {
			steeredReqs[i] = append(steeredReqs[i], shardSteered[i]...)
		}
	}
	return steeredReqs
}
