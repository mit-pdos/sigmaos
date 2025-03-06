package state

import (
	"slices"

	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

// State cache which knows all information N replicas instantaneously.  It
// initially shards the instances, and from then on adjusts the shard based on
// a probing function.
type TopNStateCache struct {
	t                 *uint64
	n                 int
	instances         []*simms.MicroserviceInstance
	shardProbeResults []map[int]*simms.LoadBalancerProbeResult
	shards            [][]int
	shard             simms.LoadBalancerShardFn
	probe             simms.LoadBalancerProbeFn
	getMetric         simms.LoadBalancerMetricProbeFn
}

func NewTopNStateCache(t *uint64, n int, shard simms.LoadBalancerShardFn, probe simms.LoadBalancerProbeFn, getMetric simms.LoadBalancerMetricProbeFn) simms.LoadBalancerStateCache {
	return &TopNStateCache{
		t:                 t,
		n:                 n,
		instances:         nil,
		shardProbeResults: nil,
		shards:            nil,
		shard:             shard,
		probe:             probe,
		getMetric:         getMetric,
	}
}

func (c *TopNStateCache) init() {
	shards := c.shard(c.instances)
	c.shards = shards
	c.shardProbeResults = make([]map[int]*simms.LoadBalancerProbeResult, len(shards))
	for i, shard := range shards {
		c.shardProbeResults[i] = make(map[int]*simms.LoadBalancerProbeResult)
		for _, instanceIdx := range shard {
			c.shardProbeResults[i][instanceIdx] = simms.NewLoadBalancerProbeResult(instanceIdx, 0)
		}
	}
}

// Update a shard given new probe results. Only keep the best N probe results,
// where "best" means lowest metric value.
func (c *TopNStateCache) updateShard(shardIdx int, shardResults []*simms.LoadBalancerProbeResult) {
	// TODO: rework this to use an interface "Less" function, same as the metrics
	// interface
	newShardProbeResults := make(map[int]*simms.LoadBalancerProbeResult)
	// Deduplicate any probe results by throwing all probe results we know about
	// (old and new) into a map
	for instanceIdx, oldResult := range c.shardProbeResults[shardIdx] {
		newShardProbeResults[instanceIdx] = oldResult
	}
	for _, newResult := range shardResults {
		newShardProbeResults[newResult.InstanceIdx] = newResult
	}
	bestN := make([]*simms.LoadBalancerProbeResult, 0, len(newShardProbeResults))
	for _, res := range newShardProbeResults {
		bestN = append(bestN, res)
	}
	slices.SortFunc(bestN, func(a, b *simms.LoadBalancerProbeResult) int {
		return a.Stat - b.Stat
	})
	var removed []*simms.LoadBalancerProbeResult
	// Truncate the slice to save only the top N results
	if len(bestN) > c.n {
		nExtra := len(bestN) - c.n
		removed = bestN[:nExtra]
		bestN = bestN[nExtra:]
	}
	for _, res := range removed {
		delete(newShardProbeResults, res.InstanceIdx)
	}
	// Update the map of probe results
	c.shardProbeResults[shardIdx] = newShardProbeResults
	// Update the slice of instance indexes in this shard
	idx := 0
	for _, res := range bestN {
		c.shards[shardIdx][idx] = res.InstanceIdx
		idx++
	}
	slices.Sort(c.shards[shardIdx])
}

func (c *TopNStateCache) GetStat(shard, instanceIdx int) int {
	return c.getMetric(c.instances[instanceIdx])
}

func (c *TopNStateCache) RunProbes(instances []*simms.MicroserviceInstance) {
	// Update the slice of instances
	c.instances = instances
	if c.shardProbeResults == nil {
		// Initialize the state cache by setting up an initial set of shards.
		c.init()
	} else {
		probeResults := c.probe(c.getMetric, instances, c.shards)
		for shardIdx := range probeResults {
			c.updateShard(shardIdx, probeResults[shardIdx])
		}
		db.DPrintf(db.ALWAYS, "Shards: %v", c.shards)
	}
}

func (c *TopNStateCache) GetShards() [][]int {
	return c.shards
}
