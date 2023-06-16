package main

import (
	"os"
	db "sigmaos/debug"
	"sigmaos/graph"
	"strconv"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v public jobname graph.", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := graph.StartThread(public, os.Args[2], os.Args[3]); err != nil {
		db.DFatalf("StartThread %v err %v\n", os.Args[0], err)
	}
}
