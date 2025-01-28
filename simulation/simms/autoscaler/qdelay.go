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
	if len(qdelays) == 0 {
		return 0.0
	}
	avg, err := stats.Mean(qdelays)
	if err != nil {
		db.DFatalf("Err calculate avg qdelay: %v", err)
	}
	return avg
}

// Calculate the average queueing delay across a set of ready service instances, for a
// given window of ticks
func AvgQDelay(ctx *Ctx, currentT uint64, windowSize uint64, istats []*simms.ServiceInstanceStats) float64 {
	if currentT < windowSize {
		db.DFatalf("Calculate avg qdelay for window of size > current time: %v > %v", windowSize, currentT)
	}
	qdelays := make([]float64, 0, len(istats))
	for _, istat := range istats {
		qdelays = append(qdelays, avgInstanceStatValInWindow(currentT-windowSize, currentT, istat, getInstanceAvgQDelay))
	}
	db.DPrintf(db.SIM_AUTOSCALE, "%v Instance avg qdelays: %v", ctx, qdelays)
	qdelay := 0.0
	for _, u := range qdelays {
		qdelay += u
	}
	qdelay /= float64(len(istats))
	return qdelay
}
