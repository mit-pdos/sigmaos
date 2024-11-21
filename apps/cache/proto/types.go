package proto

import (
	"sigmaos/apps/cache"
)

func (sa *ShardRequest) Tshard() cache.Tshard {
	return cache.Tshard(sa.Shard)
}

func (cr *CacheRequest) Tshard() cache.Tshard {
	return cache.Tshard(cr.Shard)
}
