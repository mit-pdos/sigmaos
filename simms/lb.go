package simms

type NewLoadBalancerFn func() LoadBalancer

type LoadBalancer interface {
	SteerRequests([]*Request, []*MicroserviceInstance) [][]*Request
}
