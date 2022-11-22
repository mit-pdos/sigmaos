package main

import (
	"os"

	"sigmaos/dbd"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v dbdaddr", os.Args[0])
	}
	if err := dbd.RunDbd(os.Args[1]); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
