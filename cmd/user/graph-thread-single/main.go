package main

import (
	"os"
	db "sigmaos/debug"
	"sigmaos/graph"
	"strconv"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v public jobname.", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := graph.StartThreadSingle(public, os.Args[2]); err != nil {
		db.DFatalf("StartThreadSingle %v err %v\n", os.Args[0], err)
	}
}
