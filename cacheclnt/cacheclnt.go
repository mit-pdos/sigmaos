package cacheclnt

import (
	"hash/fnv"

	"sigmaos/fslib"
	"sigmaos/hotel"
	"sigmaos/protdevclntgrp"
)

func key2shard(key string, nshard int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := int(h.Sum32()) % nshard
	return shard
}

type CacheClnt struct {
	*protdevclntgrp.ClntGroup
}

func MkCacheClnt(fsl *fslib.FsLib, n int) (*CacheClnt, error) {
	cc := &CacheClnt{}
	cg, err := protdevclntgrp.MkProtDevClntGrp(fsl, n)
	if err != nil {
		return nil, err
	}
	cc.ClntGroup = cg
	return cc, nil
}

func (gc *CacheClnt) RPC(m string, arg hotel.CacheRequest, res any) error {
	n := key2shard(arg.Key, gc.Nshard())
	return gc.ClntGroup.RPC(n, m, arg, res)
}
