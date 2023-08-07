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
	*shardsvcmgr.ServerMgr
	job string
}

func MkCacheMgr(sc *sigmaclnt.SigmaClnt, job string, nsrv int, mcpu proc.Tmcpu, gc, public bool) (*CacheMgr, error) {
	cm := &CacheMgr{}
	sm, err := shardsvcmgr.MkServerMgr(sc, nsrv, mcpu, job, "cached", CACHE, gc, public)
	if err != nil {
		return nil, err
	}
	cm.ServerMgr = sm
	return cm, nil
}
