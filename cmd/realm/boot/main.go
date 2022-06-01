package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/realm"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	e := realm.MakeTestEnv()
	_, err := e.Boot()
	if err != nil {
		db.DFatalf("Boot %v\n", err)
	}
}
