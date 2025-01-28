package autoscaler

import (
	"sigmaos/simulation/simms"
)

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
