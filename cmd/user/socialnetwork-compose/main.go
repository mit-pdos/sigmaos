package main

import (
	"os"
	dbg "sigmaos/debug"
	sn "sigmaos/socialnetwork"
	"strconv"
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
	if err := sn.RunComposeSrv(public, os.Args[2]); err != nil {
		dbg.DFatalf("RunComposeSrv %v err %v\n", os.Args[0], err)
	}
}
