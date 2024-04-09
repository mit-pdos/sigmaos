package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/keys"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv"
)

func main() {
	if len(os.Args) != 11 {
		db.DFatalf("Usage: %v masterPublicKey pubkey privkey kernelId sigmaclntdPID masterPublicKey scdpubkey scdprivkey port\nPassed: %v", os.Args[0], os.Args)
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(os.Args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys", err)
	}
	netproxy, err := strconv.ParseBool(os.Args[5])
	if err != nil {
		db.DFatalf("Can't parse netproxy bool: %v", err)
	}
	scPID := sp.Tpid(os.Args[6])
	// ignore scheddIp
	if err := uprocsrv.RunUprocSrv(os.Args[4], netproxy, os.Args[10], scPID, os.Args[7:10], masterPubKey, pubkey, privkey); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
