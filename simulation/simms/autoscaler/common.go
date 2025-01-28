package autoscaler

import (
	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

// Calculate the average util of an instance within a window [startT, endT]
func avgInstanceUtilInWindow(startT uint64, endT uint64, istat *simms.ServiceInstanceStats) float64 {
	n := 0
	util := 0.0
	for t := startT; t <= endT; t++ {
		// If instance was ready at time t, include util info in calculation
		if istat.Ready[t] {
			util += istat.Util[t]
			n++
		}
	}
	util /= float64(n)
	return util
}

// Calculate the average util across a set of ready service instances, for a
// given window of ticks
func AvgUtil(ctx *Ctx, currentT uint64, windowSize uint64, istats []*simms.ServiceInstanceStats) float64 {
	if currentT < windowSize {
		db.DFatalf("Calculate avg util for window of size > current time: %v > %v", windowSize, currentT)
	}
	utils := make([]float64, 0, len(istats))
	for _, istat := range istats {
		utils = append(utils, avgInstanceUtilInWindow(currentT-windowSize, currentT, istat))
	}
	db.DPrintf(db.SIM_AUTOSCALE, "%v Instance avg utils: %v", ctx, utils)
	util := 0.0
	for _, u := range utils {
		util += u
	}
	util /= float64(len(istats))
	return util
}

// Get any instances which are currently marked as ready
func getReadyInstanceStats(t uint64, istats []*simms.ServiceInstanceStats) []*simms.ServiceInstanceStats {
	st := make([]*simms.ServiceInstanceStats, 0, len(istats))
	for _, istat := range istats {
		if istat.Ready[t] {
			st = append(st, istat)
		}
	}
	return st
}
