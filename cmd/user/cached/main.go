package main

import (
	"os"
	"strconv"

	"sigmaos/apps/cache"
	cachesrv "sigmaos/apps/cache/srv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v cachedir [shrdpn] useEPCache", os.Args[0])
	}
	useEPCache, err := strconv.ParseBool(os.Args[3])
	if err != nil {
		db.DFatalf("Err parse useEPCache: %v", err)
	}
	if err := cachesrv.RunCacheSrv(os.Args, cache.NSHARD, useEPCache); err != nil {
		db.DFatalf("Start %v err %v\n", os.Args[0], err)
	}
}
