package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/realm"
)

func main() {
	if len(os.Args) < 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	r := realm.MakeSigmaResourceMgr()
	r.Work()
}
