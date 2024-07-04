package autoscaler

type scalingDecision int

const (
	SCALE_UP scalingDecision = iota + 1
	SCALE_DOWN
	SCALE_NONE
)

const (
	DEFAULT_TOLERANCE float64 = 0.1
)
