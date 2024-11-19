package main

import (
	"os"
	sn "sigmaos/apps/socialnetwork"
	dbg "sigmaos/debug"
	"strconv"
)

func main() {
	if len(os.Args) != 3 {
		dbg.DFatalf("Usage: %v jobname public", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[2])
	if err != nil {
		dbg.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := sn.RunFrontendSrv(public, os.Args[1]); err != nil {
		dbg.DFatalf("RunFrontendSrv %v err %v\n", os.Args[0], err)
	}
}
