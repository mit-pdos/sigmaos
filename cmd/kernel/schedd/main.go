package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/schedd"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v kernelId", os.Args[0])
	}
	schedd.RunSchedd(os.Args[1])
}
