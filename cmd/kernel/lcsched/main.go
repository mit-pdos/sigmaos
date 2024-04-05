package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/lcschedsrv"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v masterPubKey pubKey privKey\n", os.Args[0])
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	lcschedsrv.Run(masterPubKey, pubkey, privkey)
}
