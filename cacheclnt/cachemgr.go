package cacheclnt

import (
	"sigmaos/cachedsvc"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	CACHEREL = "cache"
	CACHE    = sp.NAMED + CACHEREL + "/"
)

type CacheMgr struct {
	*cachedsvc.CachedSvc
	job string
}

func MkCacheMgr(sc *sigmaclnt.SigmaClnt, job string, nsrv int, mcpu proc.Tmcpu, gc, public bool) (*CacheMgr, error) {
	cm := &CacheMgr{}
	sm, err := cachedsvc.MkCachedSvc(sc, nsrv, mcpu, job, "cached", CACHE, gc, public)
	if err != nil {
		return nil, err
	}
	cm.CachedSvc = sm
	return cm, nil
}
