package main

import (
	"os"

	"sigmaos/chunksrv"
	db "sigmaos/debug"
	"sigmaos/keys"
)

func main() {
	if len(os.Args) != 5 {
		db.DFatalf("Usage: %v masterPubKey pubKey privKey kernelId %v", os.Args[0])
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	chunksrv.Run(os.Args[4], masterPubKey, pubkey, privkey)
}
