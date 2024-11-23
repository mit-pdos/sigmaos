package hotel

import (
	"fmt"

	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	"sigmaos/apps/kv"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/memcachedclnt"
)

func NewCacheClnt(cache string, fsl *fslib.FsLib, job string) (cache.CacheClnt, error) {
	switch cache {
	case "cached":
		cc := cachegrpclnt.NewCachedSvcClnt(fsl, job)
		return cc, nil
	case "kvd":
		db.DPrintf(db.ALWAYS, "cache %v\n", cache)
		cc, err := kv.NewClerkStart(fsl, job, false)
		if err != nil {
			return nil, err
		}
		db.DPrintf(db.ALWAYS, "NewClerkFsl done %v\n", cache)
		return cc, nil
	case "memcached":
		cc, err := memcachedclnt.NewMemcachedClnt(fsl, job)
		if err != nil {
			return nil, err
		}
		return cc, nil
	default:
		db.DPrintf(db.ERROR, "Unknown cache type %v", cache)
		return nil, fmt.Errorf("Unknown cache type %v", cache)
	}
}
