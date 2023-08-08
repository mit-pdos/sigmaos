package kv

import (
	"sigmaos/cacheclnt"
	"sigmaos/fslib"
)

type CacheClnt struct {
	*cacheclnt.CacheClnt
	fsl *fslib.FsLib
}

func NewCacheClnt(fsls []*fslib.FsLib, job string, nshard uint32) (*CacheClnt, error) {
	cc, err := cacheclnt.NewCacheClntRPC(fsls, job, nshard)
	if err != nil {
		return nil, err
	}
	return &CacheClnt{
		fsl:       fsls[0],
		CacheClnt: cc,
	}, nil
}
