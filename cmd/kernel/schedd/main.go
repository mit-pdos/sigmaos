package main

import (
	"os"
	"strconv"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/schedd"
)

func main() {
	if len(os.Args) != 6 {
		db.DFatalf("Usage: %v kernelId reserveMcpu masterPublicKey pubkey privkey", os.Args[0])
	}
	reserveMcpu, err := strconv.ParseUint(os.Args[2], 10, 32)
	if err != nil {
		db.DFatalf("Cannot parse reserve cpu unit \"%v\": %v", os.Args[2], err)
	}
	schedd.RunSchedd(os.Args[1], uint(reserveMcpu), auth.PublicKey(os.Args[3]), auth.PublicKey(os.Args[4]), auth.PrivateKey(os.Args[5]))
}
