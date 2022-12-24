package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/realm"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v realmId", os.Args[0])
	}
	clnt, err := realm.MakeRealmClnt()
	if err != nil {
		db.DFatalf("%v MakeRealmClnt err %v", os.Args[0], err)
	}
	clnt.CreateRealm(os.Args[1])
}
