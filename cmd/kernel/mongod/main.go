package main

import (
	"os"
	"runtime/debug"
	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/mongosrv"
)

func main() {
	// for benchmark purpose, disable gc
	debug.SetGCPercent(-1)
	if len(os.Args) != 5 {
		db.DFatalf("Usage: %v masterPubKey pubKey privKey, mongodUrl", os.Args[0])
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	if err := mongosrv.RunMongod(os.Args[1], masterPubKey, pubkey, privkey); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
