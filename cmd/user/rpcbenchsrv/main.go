package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/rpcbench"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v path public", os.Args[0])
	}
	public, err := strconv.ParseBool(os.Args[2])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := rpcbench.RunRPCBenchSrv(os.Args[0], public); err != nil {
		db.DFatalf("RunRPCBenchSrv %v err %v\n", os.Args[0], err)
	}
}
