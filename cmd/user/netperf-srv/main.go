package main

import (
	"os"

	"sigmaos/benchmarks/netperf"
	db "sigmaos/debug"
)

func main() {
	if err := netperf.RunSrv(os.Args[1:]); err != nil {
		db.DFatalf("Err RunSrv: %v", err)
	}
}
