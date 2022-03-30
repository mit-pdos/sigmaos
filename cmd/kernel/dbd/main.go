package main

import (
	"os"

	"ulambda/dbd"
	db "ulambda/debug"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	dbd.RunDbd()
}
