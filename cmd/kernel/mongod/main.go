package main

import (
	"os"
	"runtime/debug"
	dbg "sigmaos/debug"
	"sigmaos/mongosrv"
)

func main() {
	// for benchmark purpose, disable gc
	debug.SetGCPercent(-1)
	if len(os.Args) != 2 {
		dbg.DFatalf("Usage: %v mongodUrl", os.Args[0])
	}
	if err := mongosrv.RunMongod(os.Args[1]); err != nil {
		dbg.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
