package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/pyenv/srv"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v kernelId pyMgrMode", os.Args[0])
	}
	srv.Run(os.Args[1], os.Args[2])
}
