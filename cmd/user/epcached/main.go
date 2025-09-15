package main

import (
	"os"

	"sigmaos/apps/epcache/srv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	if err := srv.RunSrv(); err != nil {
		db.DFatalf("RunEPCacheSrv %v err %v\n", os.Args[0], err)
	}
}
