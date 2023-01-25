package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/uprocsrv"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v realm proctype", os.Args[0])
	}
	if err := uprocsrv.RunUprocSrv(os.Args[1], proc.ParseTtype(os.Args[2])); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
