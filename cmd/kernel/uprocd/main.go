package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/keys"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv"
)

func main() {
	if len(os.Args) != 10 {
		db.DFatalf("Usage: %v masterPublicKey pubkey privkey kernelId sigmaclntdPID masterPublicKey scdpubkey scdprivkey port\nPassed: %v", os.Args[0], os.Args)
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	scPID := sp.Tpid(os.Args[5])
	// ignore scheddIp
	if err := uprocsrv.RunUprocSrv(os.Args[4], os.Args[9], scPID, os.Args[6:9], masterPubKey, pubkey, privkey); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
