package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/realmsrv"
)

func main() {
	if len(os.Args) != 5 {
		db.DFatalf("Usage: %v masterPubKey pubKey privKey usenetproxy", os.Args[0])
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	netproxy, err := strconv.ParseBool(os.Args[4])
	if err != nil {
		db.DFatalf("Error parse netproxy: %v", err)
	}
	if err := realmsrv.RunRealmSrv(netproxy, masterPubKey, pubkey, privkey); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
