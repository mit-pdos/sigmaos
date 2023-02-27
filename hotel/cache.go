package hotel

import (
	"fmt"
	"log"

	"sigmaos/cacheclnt"
	"sigmaos/fslib"
	"sigmaos/kv"
)

type CacheClnt interface {
	Get(string, any) error
	Put(string, any) error
	IsMiss(error) bool
}

func MkCacheClnt(cache string, fsl *fslib.FsLib, job string) (CacheClnt, error) {
	if cache == "cached" {
		cc, err := cacheclnt.MkCacheClnt(fsl, job)
		if err != nil {
			return nil, err
		}
		return cc, nil
	} else {
		log.Printf("cache %v\n", cache)
		cc, err := kv.MakeClerkFsl(fsl, job)
		if err != nil {
			return nil, err
		}
		log.Printf("MakeClerkFsl done %v\n", cache)
		return cc, nil
	}
	return nil, fmt.Errorf("Unknown cache")
}
