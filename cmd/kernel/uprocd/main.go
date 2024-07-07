package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv"
)

func main() {
	if len(os.Args) != 5 {
		db.DFatalf("Usage: %v kernelId netproxy spproxydPID port\nPassed: %v", os.Args[0], os.Args)
	}
	netproxy, err := strconv.ParseBool(os.Args[2])
	if err != nil {
		db.DFatalf("Can't parse netproxy bool: %v", err)
	}
	scPID := sp.Tpid(os.Args[3])
	// ignore scheddIp
	if err := uprocsrv.RunUprocSrv(os.Args[1], netproxy, os.Args[4], scPID); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
