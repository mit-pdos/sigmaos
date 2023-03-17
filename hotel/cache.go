package hotel

import (
	"fmt"
	"log"

	//	"google.golang.org/protobuf/proto"

	"sigmaos/cache"
	"sigmaos/cacheclnt"
	"sigmaos/fslib"
	"sigmaos/kv"
)

func MkCacheClnt(cache string, fsl *fslib.FsLib, job string) (cache.CacheClnt, error) {
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
