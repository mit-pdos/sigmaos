package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/procqsrv"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v provider", os.Args[0])
	}
	provider := sp.ParseTprovider(os.Args[1])
	procqsrv.Run(provider)
}
