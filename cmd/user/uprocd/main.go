package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/uprocsrv"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v realm\n", os.Args[0])
	}
	if err := uprocsrv.RunUprocSrv(os.Args[1]); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
