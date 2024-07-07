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
