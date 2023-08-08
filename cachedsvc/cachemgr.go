package cachedsvc

import (
	"sigmaos/cache"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

type CacheMgr struct {
	*CachedSvc
	job string
}

func MkCacheMgr(sc *sigmaclnt.SigmaClnt, job string, nsrv int, mcpu proc.Tmcpu, gc, public bool) (*CacheMgr, error) {
	cm := &CacheMgr{}
	sm, err := MkCachedSvc(sc, nsrv, mcpu, job, "cached", cache.CACHE, gc, public)
	if err != nil {
		return nil, err
	}
	cm.CachedSvc = sm
	return cm, nil
}
