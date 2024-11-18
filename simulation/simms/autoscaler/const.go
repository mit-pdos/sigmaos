package autoscaler

import (
	db "sigmaos/debug"
)

type scalingDecision int

const (
	SCALE_UP scalingDecision = iota + 1
	SCALE_DOWN
	SCALE_NONE
)

const (
	DEFAULT_TOLERANCE float64 = 0.1
)

func (sd scalingDecision) String() string {
	switch sd {
	case SCALE_UP:
		return "SCALE_UP"
	case SCALE_DOWN:
		return "SCALE_DOWN"
	case SCALE_NONE:
		return "SCALE_NONE"
	default:
		db.DFatalf("Unknown scaling decision %d", int(sd))
		return ""
	}
}
