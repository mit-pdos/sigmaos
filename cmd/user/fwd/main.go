package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/fwsrv"
)

func main() {
	// XXX no need for jobname; should have one per realm, and should
	// be a kernel service.
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v jobname", os.Args[0])
	}
	if err := fwsrv.RunFireWall(); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
