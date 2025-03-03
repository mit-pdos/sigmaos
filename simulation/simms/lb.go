package simms

type NewLoadBalancerFn func(NewLoadBalancerMetricFn, NewLoadBalancerShardingFn) LoadBalancer

type LoadBalancer interface {
	SteerRequests([]*Request, []*MicroserviceInstance) [][]*Request
}

type LoadBalancerStateCache interface {
	GetStat(shard int, t int) int
}

type NewLoadBalancerMetricFn func(lbStateCache LoadBalancerStateCache, steeredReqsPerShard [][][]*Request, instances []*MicroserviceInstance) LoadBalancerMetric

type LoadBalancerMetric interface {
	Less(shard, i, j int) bool
}

type NewLoadBalancerShardingFn func(instances []*MicroserviceInstance) [][]int
type LoadBalancerInstanceChoiceFn func(m LoadBalancerMetric, shardIdx int, shards [][]int) int
