package main

import (
	"os"
	"strconv"

	"sigmaos/apps/cache"
	cachesrv "sigmaos/apps/cache/srv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 7 {
		db.DFatalf("Usage: %v cachedir jobname srvpn useEPCache oldNSrv newNSrv", os.Args[0])
	}
	cachedir := os.Args[1]
	jobname := os.Args[2]
	srvpn := os.Args[3]
	useEPCache, err := strconv.ParseBool(os.Args[4])
	if err != nil {
		db.DFatalf("Err parse useEPCache: %v", err)
	}
	oldNSrv, err := strconv.Atoi(os.Args[5])
	if err != nil {
		db.DFatalf("Err parse oldNSrv: %v", err)
	}
	newNSrv, err := strconv.Atoi(os.Args[6])
	if err != nil {
		db.DFatalf("Err parse newNSrv: %v", err)
	}
	if err := cachesrv.RunCacheSrvScaler(cachedir, jobname, srvpn, cache.NSHARD, oldNSrv, newNSrv, useEPCache); err != nil {
		db.DFatalf("Start %v err %v", os.Args[0], err)
	}
}
