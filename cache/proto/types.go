package proto

import (
	"sigmaos/cache"
)

func (sa *ShardArg) Tshard() cache.Tshard {
	return cache.Tshard(sa.Shard)
}

func (cr *CacheRequest) Tshard() cache.Tshard {
	return cache.Tshard(cr.Shard)
}
