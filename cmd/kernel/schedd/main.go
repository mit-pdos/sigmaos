package main

import (
	"os"
	"strconv"
	db "sigmaos/debug"
	"sigmaos/schedd"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v kernelId reserveMcpu", os.Args[0])
	}
	reserveMcpu, err := strconv.ParseUint(os.Args[2], 10, 32)
	if err != nil {
		db.DFatalf("Cannot parse reserve cpu unit %v", os.Args[2])
	}
	schedd.RunSchedd(os.Args[1], uint(reserveMcpu))
}
