package simms

type MicroserviceOpt interface {
	Apply(*MicroserviceOpts)
}

type MicroserviceOpts struct {
	NewQMgr                            NewQMgrFn
	NewAutoscaler                      NewAutoscalerFn
	NewLoadBalancer                    NewLoadBalancerFn
	NewLoadBalancerMetric              NewLoadBalancerMetricFn
	LoadBalancerMetricProbe            LoadBalancerMetricProbeFn
	NewLoadBalancerStateCache          NewLoadBalancerStateCacheFn
	LoadBalancerShard                  LoadBalancerShardFn
	LoadBalancerProbe                  LoadBalancerProbeFn
	LoadBalancerInstanceChoice         LoadBalancerInstanceChoiceFn
	AssignRequestsToLoadBalancerShards AssignRequestsToLoadBalancerShardsFn
	KillRemovedInstances               bool // Immediately kill removed instances (instead of draining them/waiting for them to finish before shutting an instance down)
}

func NewMicroserviceOpts(defaultOpts MicroserviceOpts, opts []MicroserviceOpt) *MicroserviceOpts {
	mopts := &MicroserviceOpts{}
	*mopts = defaultOpts
	for _, o := range opts {
		o.Apply(mopts)
	}
	return mopts
}
