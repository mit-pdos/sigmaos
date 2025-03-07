package simms

import (
	"fmt"
)

type NewLoadBalancerFn func(*uint64, LoadBalancerStateCache, NewLoadBalancerMetricFn) LoadBalancer

type LoadBalancer interface {
	SteerRequests([]*Request, []*MicroserviceInstance) [][]*Request
}

type NewLoadBalancerMetricFn func(lbStateCache LoadBalancerStateCache, steeredReqsPerShard [][][]*Request) LoadBalancerMetric

type LoadBalancerMetric interface {
	Less(shard, i, j int) bool
}

type LoadBalancerInstanceChoiceFn func(m LoadBalancerMetric, shardIdx int, shards [][]int) int

// Load balancer state
type LoadBalancerStateCache interface {
	GetStat(shard, instanceIdx int) int          // Get statistics about an instance in a shard
	RunProbes(instances []*MicroserviceInstance) // Update the cached state
	GetShards() [][]int                          // List the instances in each shard of load balancer state
}

type LoadBalancerProbeResult struct {
	InstanceIdx int
	Stat        int
}

func (pr *LoadBalancerProbeResult) String() string {
	return fmt.Sprintf("&{ instance:%v stat:%v }", pr.InstanceIdx, pr.Stat)
}

func NewLoadBalancerProbeResult(instanceIdx int, stat int) *LoadBalancerProbeResult {
	return &LoadBalancerProbeResult{
		InstanceIdx: instanceIdx,
		Stat:        stat,
	}
}

// Shard instances.
type LoadBalancerShardFn func(instances []*MicroserviceInstance) [][]int

// Probe instances
type LoadBalancerMetricProbeFn func(*MicroserviceInstance) int
type LoadBalancerProbeFn func(m LoadBalancerMetricProbeFn, instances []*MicroserviceInstance, shards [][]int) [][]*LoadBalancerProbeResult

type NewLoadBalancerStateCacheFn func(t *uint64, shard LoadBalancerShardFn, probe LoadBalancerProbeFn, m LoadBalancerMetricProbeFn) LoadBalancerStateCache
