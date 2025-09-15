package proto

import (
	"sigmaos/apps/cache"
)

func (g *CacheGetDescriptor) Tshard() cache.Tshard {
	return cache.Tshard(g.Shard)
}

func (sa *ShardReq) Tshard() cache.Tshard {
	return cache.Tshard(sa.Shard)
}

func (cr *CacheReq) Tshard() cache.Tshard {
	return cache.Tshard(cr.Shard)
}
