package simms

type MicroserviceOpt interface {
	Apply(*MicroserviceOpts)
}

// TODO: should probably separate all different parts of the simulator into
// their own sub-packages to make things cleaner
type MicroserviceOpts struct {
	NewAutoscaler         NewAutoscalerFn
	NewLoadBalancer       NewLoadBalancerFn
	NewLoadBalancerMetric NewLoadBalancerMetricFn
}

func NewMicroserviceOpts(defaultOpts MicroserviceOpts, opts []MicroserviceOpt) *MicroserviceOpts {
	mopts := &MicroserviceOpts{}
	*mopts = defaultOpts
	for _, o := range opts {
		o.Apply(mopts)
	}
	return mopts
}
