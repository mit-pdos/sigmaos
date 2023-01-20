package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/realmsrv"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v\n", os.Args[0])
	}
	if err := realmsrv.RunRealmSrv(); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
