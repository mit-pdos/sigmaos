package opts

import (
	"sigmaos/simms"
	"sigmaos/simms/lb"
)

type withRoundRobinLB struct{}

func (withRoundRobinLB) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancer = lb.NewRoundRobinLB
}

func WithRoundRobinLB() simms.MicroserviceOpt {
	return &withRoundRobinLB{}
}

type withOmniscientQLenLB struct{}

func (withOmniscientQLenLB) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancer = lb.NewOmniscientQLenLB
}

func WithOmniscientQLenLB() simms.MicroserviceOpt {
	return &withOmniscientQLenLB{}
}

type withNRandomChoicesQLenLB struct {
	n int
}

func (o withNRandomChoicesQLenLB) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancer = func() simms.LoadBalancer {
		return lb.NewNRandomChoicesQLenLB(o.n)
	}
}

func WithNRandomChoicesQLenLB(n int) simms.MicroserviceOpt {
	return &withNRandomChoicesQLenLB{
		n: n,
	}
}
