package autoscaler

import (
	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

func getInstanceUtil(t uint64, istat *simms.ServiceInstanceStats) float64 {
	return istat.Util[t]
}

// Calculate the average util across a set of ready service instances, for a
// given window of ticks
func AvgUtil(ctx *Ctx, currentT uint64, windowSize uint64, istats []*simms.ServiceInstanceStats) float64 {
	if currentT < windowSize {
		db.DFatalf("Calculate avg util for window of size > current time: %v > %v", windowSize, currentT)
	}
	utils := make([]float64, 0, len(istats))
	for _, istat := range istats {
		utils = append(utils, avgInstanceStatValInWindow(currentT-windowSize, currentT, istat, getInstanceUtil))
	}
	db.DPrintf(db.SIM_AUTOSCALE, "%v Instance avg utils: %v", ctx, utils)
	util := 0.0
	for _, u := range utils {
		util += u
	}
	util /= float64(len(istats))
	return util
}
