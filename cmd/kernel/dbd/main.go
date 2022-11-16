package main

import (
	"os"

	"sigmaos/dbd"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	if err := dbd.RunDbd(); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
