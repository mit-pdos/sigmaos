package main

import (
	"os"
	db "sigmaos/debug"
	"sigmaos/graph"
	"strconv"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v public jobname", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := graph.StartGraphSrv(public, os.Args[2]); err != nil {
		db.DFatalf("StartGraphSrv %v err %v\n", os.Args[0], err)
	}
}
