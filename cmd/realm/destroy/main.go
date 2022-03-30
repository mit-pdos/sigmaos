package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/realm"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v realmId", os.Args[0])
	}
	clnt := realm.MakeRealmClnt()
	clnt.DestroyRealm(os.Args[1])
}
