package main

import (
	"os"

	"sigmaos/chunk/srv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v kernelId %v", os.Args[0])
	}
	srv.Run(os.Args[1])
}
