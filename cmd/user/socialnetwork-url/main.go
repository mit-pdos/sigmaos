package main

import (
	"os"
	dbg "sigmaos/debug"
	sn "sigmaos/socialnetwork"
)

func main() {
	if len(os.Args) != 2 {
		dbg.DFatalf("Usage: %v jobname", os.Args[0])
		return
	}
	if err := sn.RunUrlSrv(os.Args[1]); err != nil {
		dbg.DFatalf("RunUrlSrv %v err %v\n", os.Args[0], err)
	}
}
