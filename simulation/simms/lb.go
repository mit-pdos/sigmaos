package simms

type NewLoadBalancerFn func(NewLoadBalancerMetricFn, NewLoadBalancerShardingFn) LoadBalancer

type LoadBalancer interface {
	SteerRequests([]*Request, []*MicroserviceInstance) [][]*Request
}

type NewLoadBalancerMetricFn func(steeredReqs [][]*Request, instances []*MicroserviceInstance) LoadBalancerMetric
type NewLoadBalancerShardingFn func(instances []*MicroserviceInstance) [][]int

type LoadBalancerMetric interface {
	Less(i, j int) bool
}
