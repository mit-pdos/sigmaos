package main

import (
	"os"
	"strconv"

	"sigmaos/apps/cache"
	cachesrv "sigmaos/apps/cache/srv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 6 {
		db.DFatalf("Usage: %v cachedir jobname shrdpn useEPCache topNShards", os.Args[0])
	}
	cachedir := os.Args[1]
	jobname := os.Args[2]
	shrdpn := os.Args[3]
	useEPCache, err := strconv.ParseBool(os.Args[4])
	if err != nil {
		db.DFatalf("Err parse useEPCache: %v", err)
	}
	topN, err := strconv.Atoi(os.Args[5])
	if err != nil {
		db.DFatalf("Err parse topNShards: %v", err)
	}
	if err := cachesrv.RunCacheSrvBackup(cachedir, jobname, shrdpn, cache.NSHARD, useEPCache, topN); err != nil {
		db.DFatalf("Start %v err %v", os.Args[0], err)
	}
}
