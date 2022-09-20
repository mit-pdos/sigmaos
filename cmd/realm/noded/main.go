package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/realm"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v machinedId", os.Args[0])
	}

	r := realm.MakeNoded(os.Args[1])
	r.Work()
}
