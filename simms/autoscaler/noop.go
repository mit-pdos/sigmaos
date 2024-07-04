package autoscaler

import (
	"sigmaos/simms"
)

func GetNewNoOpAutoscalerFn() simms.NewAutoscalerFn {
	return func(*simms.Microservice) simms.Autoscaler {
		return &NoOpAutoscaler{}
	}
}

type NoOpAutoscaler struct {
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
