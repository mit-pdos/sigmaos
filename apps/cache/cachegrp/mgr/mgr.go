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

func NewCacheMgr(sc *sigmaclnt.SigmaClnt, job string, cfg *CacheJobConfig) (*CacheMgr, error) {
	return NewCacheMgrEPCache(sc, nil, job, cfg)
}

func NewCacheMgrEPCache(sc *sigmaclnt.SigmaClnt, epCacheJob *epsrv.EPCacheJob, job string, cfg *CacheJobConfig) (*CacheMgr, error) {
	cm := &CacheMgr{}
	sm, err := NewCachedSvcEPCache(sc, epCacheJob, cfg, job, "cached", cache.CACHE)
	if err != nil {
		return nil, err
	}
	cm.CachedSvc = sm
	return cm, nil
}
