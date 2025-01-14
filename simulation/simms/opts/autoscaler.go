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

type withAvgUtilAutoscaler struct {
	asp *autoscaler.AvgUtilAutoscalerParams
}

func (o withAvgUtilAutoscaler) Apply(opts *simms.MicroserviceOpts) {
	opts.NewAutoscaler = func(t *uint64, svc *simms.Microservice) simms.Autoscaler {
		return autoscaler.NewAvgUtilAutoscaler(t, o.asp, svc)
	}
}

func WithAvgUtilAutoscaler(asp *autoscaler.AvgUtilAutoscalerParams) simms.MicroserviceOpt {
	return &withAvgUtilAutoscaler{
		asp: asp,
	}
}
