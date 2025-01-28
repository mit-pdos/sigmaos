package autoscaler

import (
	"sigmaos/simulation/simms"
)

type statFn func(t uint64, istat *simms.ServiceInstanceStats) float64

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

// Calculate the average value of a stat of an instance within a window [startT, endT]
func avgInstanceStatValInWindow(startT uint64, endT uint64, istat *simms.ServiceInstanceStats, getStat statFn) float64 {
	n := 0
	val := 0.0
	for t := startT; t <= endT; t++ {
		// If instance was ready at time t, include val info in calculation
		if istat.Ready[t] {
			val += getStat(t, istat)
			n++
		}
	}
	val /= float64(n)
	return val
}
