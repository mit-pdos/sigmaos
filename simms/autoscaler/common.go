package autoscaler

import (
	db "sigmaos/debug"
	"sigmaos/simms"
)

func avgUtil(istats []*simms.ServiceInstanceStats) float64 {
	// TODO
	db.DFatalf("Unimplemented")
	return 0.0
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
