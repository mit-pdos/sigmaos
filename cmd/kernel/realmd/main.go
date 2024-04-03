package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/realmsrv"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v masterPubKey pubKey privKey\n", os.Args[0])
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	if err := realmsrv.RunRealmSrv(masterPubKey, pubkey, privkey); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
