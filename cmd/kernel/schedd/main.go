package main

import (
	"os"
	db "sigmaos/debug"
	"sigmaos/schedd"
	sp "sigmaos/sigmap"
	"strconv"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v kernelId provider reserveMcpu", os.Args[0])
	}
	provider := sp.ParseTprovider(os.Args[2])
	reserveMcpu, err := strconv.ParseUint(os.Args[3], 10, 32)
	if err != nil {
		db.DFatalf("Cannot parse reserve cpu unit \"%v\": %v", os.Args[3], err)
	}
	schedd.RunSchedd(os.Args[1], provider, uint(reserveMcpu))
}
