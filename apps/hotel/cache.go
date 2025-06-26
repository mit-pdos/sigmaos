package hotel

import (
	"fmt"

	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib"
)

func NewCacheClnt(cache string, fsl *fslib.FsLib, job string) (cache.CacheClnt, error) {
	switch cache {
	case "cached":
		cc := cachegrpclnt.NewCachedSvcClnt(fsl, job)
		return cc, nil
	default:
		db.DPrintf(db.ERROR, "Unknown cache type %v", cache)
		return nil, fmt.Errorf("Unknown cache type %v", cache)
	}
}
