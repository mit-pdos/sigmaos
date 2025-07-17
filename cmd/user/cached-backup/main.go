package main

import (
	"os"

	"sigmaos/apps/cache"
	cachesrv "sigmaos/apps/cache/srv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v cachedir jobname shrdpn", os.Args[0])
	}
	cachedir := os.Args[1]
	jobname := os.Args[2]
	shrdpn := os.Args[3]
	if err := cachesrv.RunCacheSrvBackup(cachedir, jobname, shrdpn, cache.NSHARD); err != nil {
		db.DFatalf("Start %v err %v", os.Args[0], err)
	}
}
