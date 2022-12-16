package cacheclnt

import (
	"sigmaos/fslib"
	"sigmaos/procclnt"
	"sigmaos/shardsvcmgr"
	sp "sigmaos/sigmap"
)

const (
	CACHE_NCORE = 1
)

type CacheMgr struct {
	*shardsvcmgr.ShardMgr
	job string
}

func MkCacheMgr(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, job string, n int) (*CacheMgr, error) {
	cm := &CacheMgr{}
	sm, err := shardsvcmgr.MkShardMgr(fsl, pclnt, n, job, "user/cached", sp.CACHE)
	if err != nil {
		return nil, err
	}
	cm.ShardMgr = sm
	return cm, nil
}
