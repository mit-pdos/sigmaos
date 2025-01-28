package autoscaler

import (
	"github.com/montanaflynn/stats"

	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

func getInstanceAvgQDelay(t uint64, istat *simms.ServiceInstanceStats) float64 {
	qd := istat.QDelay[t]
	qdelays := make([]float64, 0, len(qd))
	for _, d := range qd {
		qdelays = append(qdelays, float64(d))
	}
	avg, err := stats.Mean(qdelays)
	if err != nil {
		db.DFatalf("Err calculate avg qdelay: %v", err)
	}
	return avg
}

// Calculate the average util across a set of ready service instances, for a
// given window of ticks
func AvgQDelay(ctx *Ctx, currentT uint64, windowSize uint64, istats []*simms.ServiceInstanceStats) float64 {
	if currentT < windowSize {
		db.DFatalf("Calculate avg qdelay for window of size > current time: %v > %v", windowSize, currentT)
	}
	utils := make([]float64, 0, len(istats))
	for _, istat := range istats {
		utils = append(utils, avgInstanceStatValInWindow(currentT-windowSize, currentT, istat, getInstanceAvgQDelay))
	}
	db.DPrintf(db.SIM_AUTOSCALE, "%v Instance avg qdelays: %v", ctx, utils)
	util := 0.0
	for _, u := range utils {
		util += u
	}
	util /= float64(len(istats))
	return util
}
