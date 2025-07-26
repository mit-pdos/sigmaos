package mgr

import (
	"sigmaos/apps/cache"
	epsrv "sigmaos/apps/epcache/srv"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

type CacheMgr struct {
	*CachedSvc
	job string
}

func NewCacheMgr(sc *sigmaclnt.SigmaClnt, job string, nsrv int, mcpu proc.Tmcpu, gc bool) (*CacheMgr, error) {
	return NewCacheMgrEPCache(sc, nil, job, nsrv, mcpu, gc)
}

func NewCacheMgrEPCache(sc *sigmaclnt.SigmaClnt, epCacheJob *epsrv.EPCacheJob, job string, nsrv int, mcpu proc.Tmcpu, gc bool) (*CacheMgr, error) {
	cm := &CacheMgr{}
	sm, err := NewCachedSvcEPCache(sc, epCacheJob, nsrv, mcpu, job, "cached", cache.CACHE, gc)
	if err != nil {
		return nil, err
	}
	cm.CachedSvc = sm
	return cm, nil
}
