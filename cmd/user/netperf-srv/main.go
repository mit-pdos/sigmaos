package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/netperf"
)

func main() {
	if err := netperf.RunSrv(os.Args[1:]); err != nil {
		db.DFatalf("Err RunSrv: %v", err)
	}
}
