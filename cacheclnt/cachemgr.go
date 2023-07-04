package cacheclnt

import (
	"sigmaos/proc"
	"sigmaos/shardsvcmgr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	CACHEREL = "cache"
	CACHE    = sp.NAMED + CACHEREL + "/"
)

type CacheMgr struct {
	*shardsvcmgr.ShardMgr
	job string
}

func MkCacheMgr(sc *sigmaclnt.SigmaClnt, job string, n int, mcpu proc.Tmcpu, gc, public bool) (*CacheMgr, error) {
	cm := &CacheMgr{}
	sm, err := shardsvcmgr.MkShardMgr(sc, n, mcpu, job, "cached", CACHE, gc, public)
	if err != nil {
		return nil, err
	}
	cm.ShardMgr = sm
	return cm, nil
}
