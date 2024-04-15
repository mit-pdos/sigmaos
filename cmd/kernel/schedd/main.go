package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/schedsrv"
)

func main() {
	if len(os.Args) != 6 {
		db.DFatalf("Usage: %v masterPublicKey pubkey privkey kernelId reserveMcpu", os.Args[0])
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	reserveMcpu, err := strconv.ParseUint(os.Args[5], 10, 32)
	if err != nil {
		db.DFatalf("Cannot parse reserve cpu unit \"%v\": %v", os.Args[5], err)
	}
	schedsrv.RunSchedd(os.Args[4], uint(reserveMcpu), masterPubKey, pubkey, privkey)
}
