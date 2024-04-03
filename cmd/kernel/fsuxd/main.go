package main

import (
	"os"
	"path"

	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/ux"
)

func main() {
	if len(os.Args) != 5 {
		db.DFatalf("Usage: %v masterPubKey pubKey privKey rootux", os.Args[0])
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	rootux := os.Args[4]
	db.DPrintf(db.UX, "root ux %v\n", rootux)
	if err := os.MkdirAll(path.Join(rootux, "bin", "user"), 0755); err != nil {
		db.DFatalf("Error MkdirAll: %v", err)
	}
	fsux.RunFsUx(rootux, masterPubKey, pubkey, privkey)
}
