package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/realm"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v realmId", os.Args[0])
	}
	r := realm.MakeRealmResourceMgr(os.Args[1])
	r.Work()
}
