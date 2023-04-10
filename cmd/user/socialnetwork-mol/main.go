package main

import (
	"os"
	"strconv"
	dbg "sigmaos/debug"
	"sigmaos/socialnetwork"
)

func main() {
	if len(os.Args) != 3 {
		dbg.DFatalf("Usage: %v public jobname", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		dbg.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := socialnetwork.RunMoLSrv(public); err != nil {
		dbg.DFatalf("RunMoLSrv %v err %v\n", os.Args[0], err)
	}
}
