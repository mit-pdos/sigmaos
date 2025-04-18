package main

import (
	"os"
	sn "sigmaos/apps/socialnetwork"
	dbg "sigmaos/debug"
)

func main() {
	if len(os.Args) != 2 {
		dbg.DFatalf("Usage: %v jobname", os.Args[0])
		return
	}
	if err := sn.RunComposeSrv(os.Args[1]); err != nil {
		dbg.DFatalf("RunComposeSrv %v err %v\n", os.Args[0], err)
	}
}
