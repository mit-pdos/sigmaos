package metrics

import (
	"sigmaos/simms"
)

type NewMetricFn func(steeredReqs [][]*simms.Request, instances []*simms.MicroserviceInstance)

type Metric interface {
	Less(i, j int) bool
}
