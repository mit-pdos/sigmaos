package opts

import (
	"sigmaos/simulation/simms"
	"sigmaos/simulation/simms/lb"
	lbchoice "sigmaos/simulation/simms/lb/choice"
	lbmetrics "sigmaos/simulation/simms/lb/metrics"
	lbprobe "sigmaos/simulation/simms/lb/probe"
	lbshard "sigmaos/simulation/simms/lb/shard"
	lbstate "sigmaos/simulation/simms/lb/state"
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
	opts.NewLoadBalancer = func(t *uint64, lbStateCache simms.LoadBalancerStateCache, newMetric simms.NewLoadBalancerMetricFn, assignReqs simms.AssignRequestsToLoadBalancerShardsFn) simms.LoadBalancer {
		return lb.NewOmniscientLB(t, lbStateCache, newMetric, lbchoice.FullScan, assignReqs)
	}
}

func WithOmniscientLB() simms.MicroserviceOpt {
	return &withOmniscientLB{}
}

type withCachedStateLB struct {
	probesPerTick int
}

func (o withCachedStateLB) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancer = func(t *uint64, lbStateCache simms.LoadBalancerStateCache, newMetric simms.NewLoadBalancerMetricFn, assignReqs simms.AssignRequestsToLoadBalancerShardsFn) simms.LoadBalancer {
		return lb.NewCachedStateLB(t, lbStateCache, newMetric, lbchoice.FullScan, assignReqs)
	}
}

func WithCachedStateLB(probesPerTick int) simms.MicroserviceOpt {
	return &withCachedStateLB{
		probesPerTick: probesPerTick,
	}
}

type withNRandomChoicesLB struct {
	n int
}

func (o withNRandomChoicesLB) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancer = func(t *uint64, lbStateCache simms.LoadBalancerStateCache, newMetric simms.NewLoadBalancerMetricFn, assignReqs simms.AssignRequestsToLoadBalancerShardsFn) simms.LoadBalancer {
		return lb.NewOmniscientLB(t, lbStateCache, newMetric, func(m simms.LoadBalancerMetric, shardIdx int, shards [][]int) int {
			return lbchoice.RandomSubset(m, shardIdx, shards, o.n)
		}, assignReqs)
	}
}

func WithNRandomChoicesLB(n int) simms.MicroserviceOpt {
	return &withNRandomChoicesLB{
		n: n,
	}
}

type withDeterministicSubsettingLBShards struct {
	shard simms.LoadBalancerShardFn
}

func (o withDeterministicSubsettingLBShards) Apply(opts *simms.MicroserviceOpts) {
	opts.LoadBalancerShard = o.shard
}

func WithDeterministicSubsettingLBShards(nshards, nInstancesPerShard int) simms.MicroserviceOpt {
	return &withDeterministicSubsettingLBShards{
		shard: func(instances []*simms.MicroserviceInstance) [][]int {
			return lbshard.SelectDeterministicSubsettingShards(instances, nshards, nInstancesPerShard)
		},
	}
}

type withRandomLBShards struct {
	shard simms.LoadBalancerShardFn
}

func (o withRandomLBShards) Apply(opts *simms.MicroserviceOpts) {
	opts.LoadBalancerShard = o.shard
}

func WithRandomNonOverlappingLBShards(nshards int) simms.MicroserviceOpt {
	return &withRandomLBShards{
		shard: func(instances []*simms.MicroserviceInstance) [][]int {
			return lbshard.SelectNonOverlappingRandomShards(instances, nshards)
		},
	}
}

func WithRandomOverlappingLBShards(nshards int, nInstancesPerShard int) simms.MicroserviceOpt {
	return &withRandomLBShards{
		shard: func(instances []*simms.MicroserviceInstance) [][]int {
			shardSize := min(nInstancesPerShard, len(instances))
			return lbshard.SelectOverlappingRandomShards(instances, nshards, shardSize)
		},
	}
}

type withTopNLBStateCache struct {
	n int
}

func (o withTopNLBStateCache) Apply(opts *simms.MicroserviceOpts) {
	opts.NewLoadBalancerStateCache = func(t *uint64, shard simms.LoadBalancerShardFn, probe simms.LoadBalancerProbeFn, getMetric simms.LoadBalancerMetricProbeFn) simms.LoadBalancerStateCache {
		return lbstate.NewTopNStateCache(t, o.n, shard, probe, getMetric)
	}
}

func WithTopNLBStateCache(n int) simms.MicroserviceOpt {
	return &withTopNLBStateCache{
		n: n,
	}
}

type withNOldNNewLBProbes struct {
	all  bool
	nOld int
	nNew int
}

func (o withNOldNNewLBProbes) Apply(opts *simms.MicroserviceOpts) {
	opts.LoadBalancerProbe = func(m simms.LoadBalancerMetricProbeFn, instances []*simms.MicroserviceInstance, shards [][]int) [][]*simms.LoadBalancerProbeResult {
		if o.all {
			return lbprobe.ProbeAllPlusNNew(o.nNew, m, instances, shards)
		} else {
		}
		return lbprobe.ProbeNOldPlusNNew(o.nOld, o.nNew, m, instances, shards)
	}
}

func WithNOldPlusNNewLBProbes(nOld int, nNew int) simms.MicroserviceOpt {
	return &withNOldNNewLBProbes{
		all:  false,
		nOld: nOld,
		nNew: nNew,
	}
}

func WithNNewLBProbes(n int) simms.MicroserviceOpt {
	return &withNOldNNewLBProbes{
		all:  true,
		nNew: n,
		nOld: -1,
	}
}

type withGaussianRequestToLBShardAssignment struct {
}

func (o withGaussianRequestToLBShardAssignment) Apply(opts *simms.MicroserviceOpts) {
	opts.AssignRequestsToLoadBalancerShards = lbshard.AssignRequestsToShardsGaussian
}

func WithGaussianRequestToLBShardAssignment() simms.MicroserviceOpt {
	return &withGaussianRequestToLBShardAssignment{}
}
