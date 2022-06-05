package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/realm"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v realmid", os.Args[0])
	}
	e := realm.MakeTestEnv(os.Args[1])
	_, err := e.Boot()
	if err != nil {
		db.DFatalf("Boot %v\n", err)
	}
}
