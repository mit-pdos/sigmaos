package main

import (
	"os"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv"
)

func main() {
	if len(os.Args) != 7 {
		db.DFatalf("Usage: %v kernelId sigmaclntdPID masterPublicKey pubkey privkey port\nPassed: %v", os.Args[0], os.Args)
	}
	scPID := sp.Tpid(os.Args[2])
	// ignore scheddIp
	if err := uprocsrv.RunUprocSrv(os.Args[1], os.Args[6], scPID, os.Args[3:6]); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
