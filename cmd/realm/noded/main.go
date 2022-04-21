package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/realm"
)

func main() {
	if len(os.Args) < 3 {
		db.DFatalf("Usage: %v bin id", os.Args[0])
	}

	r := realm.MakeNoded(os.Args[1], os.Args[2])
	r.Work()
}
