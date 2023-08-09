package proto

import (
	"sigmaos/cache"
	sp "sigmaos/sigmap"
)

func (sa *ShardArg) Tshard() cache.Tshard {
	return cache.Tshard(sa.Shard)
}

func (cr *CacheRequest) Tshard() cache.Tshard {
	return cache.Tshard(cr.Shard)
}

func (sa *ShardArg) Tseqno() sp.Tseqno {
	return sp.Tseqno(sa.Seqno)
}

func (cr *CacheRequest) Tseqno() sp.Tseqno {
	return sp.Tseqno(cr.Seqno)
}

func (sa *ShardArg) TclntId() sp.TclntId {
	return sp.TclntId(sa.ClntId)
}

func (cr *CacheRequest) TclntId() sp.TclntId {
	return sp.TclntId(cr.ClntId)
}
