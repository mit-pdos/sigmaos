package autoscaler

import (
	"sigmaos/simms"
)

type NoOpAutoscaler struct {
}

func NewNoOpAutoscaler(*uint64, *simms.Microservice) simms.Autoscaler {
	return &NoOpAutoscaler{}
}

func (noop *NoOpAutoscaler) Tick() {
}

func (noop *NoOpAutoscaler) Start() {
}

func (noop *NoOpAutoscaler) Stop() {
}

func (noop *NoOpAutoscaler) NScaleUpEvents() int {
	return 0
}

func (noop *NoOpAutoscaler) NScaleDownEvents() int {
	return 0
}
