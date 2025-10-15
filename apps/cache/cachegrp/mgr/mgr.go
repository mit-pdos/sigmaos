package mgr

import (
	"fmt"

	"sigmaos/apps/cache"
	epsrv "sigmaos/apps/epcache/srv"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

type CacheJobConfig struct {
	NSrv int         `json:"n_srv"`
	MCPU proc.Tmcpu `json:"mcpu"`
	GC   bool        `json:"gc"`
}

func NewCacheJobConfig(nsrv int, mcpu proc.Tmcpu, gc bool) *CacheJobConfig {
	return &CacheJobConfig{
		NSrv: nsrv,
		MCPU: mcpu,
		GC:   gc,
	}
}

func (cfg *CacheJobConfig) String() string {
	return fmt.Sprintf("&{ NSrv:%v MCPU:%v GC:%v }", cfg.NSrv, cfg.MCPU, cfg.GC)
}

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
