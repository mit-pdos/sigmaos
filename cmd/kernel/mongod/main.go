package main

import (
	"os"
	"runtime/debug"
	db "sigmaos/debug"
	mongosrv "sigmaos/mongo/srv"
)

func main() {
	// for benchmark purpose, disable gc
	debug.SetGCPercent(-1)
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v mongodUrl", os.Args[0])
	}
	if err := mongosrv.RunMongod(os.Args[1]); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
