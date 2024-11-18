package main

import (
	"os"

	"sigmaos/apps/cache"
	cachesrv "sigmaos/apps/cache/srv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v cachedir [shrdpn]", os.Args[0])
	}
	if err := cachesrv.RunCacheSrv(os.Args, cache.NSHARD); err != nil {
		db.DFatalf("Start %v err %v\n", os.Args[0], err)
	}
}
