package main

import (
	"os"
	"sigmaos/mongod"
	dbg "sigmaos/debug"
)

func main() {
	if len(os.Args) != 2 {
		dbg.DFatalf("Usage: %v mongodUrl", os.Args[0])
	}
	if err := mongod.RunMongod(os.Args[1]); err != nil {
		dbg.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
