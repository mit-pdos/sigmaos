package main

import (
	"os"
	"strings"

	db "ulambda/debug"
	"ulambda/realm"
)

func main() {
	if len(os.Args) < 3 {
		db.DFatalf("Usage: %v realmId sigmaNamedAddrs", os.Args[0])
	}
	r := realm.MakeRealmResourceMgr(os.Args[1], strings.Split(os.Args[2], ","))
	r.Work()
}
