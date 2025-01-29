package opts

import (
	"sigmaos/simulation/simms"
	"sigmaos/simulation/simms/autoscaler"
)

type withNoOpAutoscaler struct{}

func (withNoOpAutoscaler) Apply(opts *simms.MicroserviceOpts) {
	opts.NewAutoscaler = autoscaler.NewNoOpAutoscaler
}

func WithNoOpAutoscaler() simms.MicroserviceOpt {
	return &withNoOpAutoscaler{}
}

type withAvgValAutoscaler struct {
	asp *autoscaler.AvgValAutoscalerParams
}

func (o withAvgValAutoscaler) Apply(opts *simms.MicroserviceOpts) {
	opts.NewAutoscaler = func(t *uint64, svc *simms.Microservice) simms.Autoscaler {
		return autoscaler.NewAvgValAutoscaler(t, o.asp, svc)
	}
}

func WithAvgValAutoscaler(asp *autoscaler.AvgValAutoscalerParams) simms.MicroserviceOpt {
	return &withAvgValAutoscaler{
		asp: asp,
	}
}
