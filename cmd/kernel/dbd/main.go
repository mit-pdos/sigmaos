package main

import (
	"os"

	"sigmaos/dbd"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	dbd.RunDbd()
}
