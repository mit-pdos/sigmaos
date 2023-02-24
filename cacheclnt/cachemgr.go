package cacheclnt

import (
	"sigmaos/shardsvcmgr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	CACHE_NCORE = 1
)

type CacheMgr struct {
	*shardsvcmgr.ShardMgr
	job string
}

func MkCacheMgr(sc *sigmaclnt.SigmaClnt, job string, n int, public bool) (*CacheMgr, error) {
	cm := &CacheMgr{}
	sm, err := shardsvcmgr.MkShardMgr(sc, n, job, "cached", sp.CACHE, public)
	if err != nil {
		return nil, err
	}
	cm.ShardMgr = sm
	return cm, nil
}
