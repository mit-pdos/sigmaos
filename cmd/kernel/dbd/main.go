package main

import (
	"os"

	"sigmaos/dbsrv"
	db "sigmaos/debug"
	"sigmaos/keys"
)

func main() {
	if len(os.Args) != 5 {
		db.DFatalf("Usage: %v dbdaddr masterPubKey pubKey privKey", os.Args[0])
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	if err := dbsrv.RunDbd(os.Args[1], masterPubKey, pubkey, privkey); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
