package simms

type NewLoadBalancerFn func(NewLoadBalancerMetricFn) LoadBalancer

type LoadBalancer interface {
	SteerRequests([]*Request, []*MicroserviceInstance) [][]*Request
}

type NewLoadBalancerMetricFn func(steeredReqs [][]*Request, instances []*MicroserviceInstance) LoadBalancerMetric

type LoadBalancerMetric interface {
	Less(i, j int) bool
}
