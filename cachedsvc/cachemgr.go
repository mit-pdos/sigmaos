package cachedsvc

import (
	"sigmaos/apps/cache"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

type CacheMgr struct {
	*CachedSvc
	job string
}

func NewCacheMgr(sc *sigmaclnt.SigmaClnt, job string, nsrv int, mcpu proc.Tmcpu, gc bool) (*CacheMgr, error) {
	cm := &CacheMgr{}
	sm, err := NewCachedSvc(sc, nsrv, mcpu, job, "cached", cache.CACHE, gc)
	if err != nil {
		return nil, err
	}
	cm.CachedSvc = sm
	return cm, nil
}
