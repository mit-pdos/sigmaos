package main

import (
	"os"

	"sigmaos/chunksrv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v kernelId %v", os.Args[0])
	}
	chunksrv.Run(os.Args[1])
}
