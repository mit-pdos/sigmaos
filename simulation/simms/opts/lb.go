package opts

import (
	"sigmaos/simulation/simms"
	"sigmaos/simulation/simms/lb"
	lbchoice "sigmaos/simulation/simms/lb/choice"
	lbmetrics "sigmaos/simulation/simms/lb/metrics"
	lbshard "sigmaos/simulation/simms/lb/shard"
)

type withRoundRobinLB struct{}

func (withRoundRobinLB) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancer = lb.NewRoundRobinLB
}

func WithRoundRobinLB() simms.MicroserviceOpt {
	return &withRoundRobinLB{}
}

type withLoadBalancerQLenMetric struct{}

func (o withLoadBalancerQLenMetric) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancerMetric = lbmetrics.NewQLenMetric
}

func WithLoadBalancerQLenMetric() simms.MicroserviceOpt {
	return &withLoadBalancerQLenMetric{}
}

type withOmniscientLB struct{}

func (withOmniscientLB) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancer = func(newMetric simms.NewLoadBalancerMetricFn, shard simms.NewLoadBalancerShardingFn) simms.LoadBalancer {
		return lb.NewOmniscientLB(newMetric, shard, lbchoice.FullScan)
	}
}

func WithOmniscientLB() simms.MicroserviceOpt {
	return &withOmniscientLB{}
}

type withNRandomChoicesLB struct {
	n int
}

func (o withNRandomChoicesLB) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancer = func(newMetric simms.NewLoadBalancerMetricFn, shard simms.NewLoadBalancerShardingFn) simms.LoadBalancer {
		return lb.NewOmniscientLB(newMetric, shard, func(m simms.LoadBalancerMetric, shardIdx int, shards [][]int) int {
			return lbchoice.RandomSubset(m, shardIdx, shards, o.n)
		})
	}
}

func WithNRandomChoicesLB(n int) simms.MicroserviceOpt {
	return &withNRandomChoicesLB{
		n: n,
	}
}

type withSingleLBShard struct{}

func (o withSingleLBShard) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancerSharding = lbshard.SingleShard
}

func WithSingleLBShard() simms.MicroserviceOpt {
	return &withSingleLBShard{}
}
