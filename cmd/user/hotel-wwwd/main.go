package main

import (
	"os"

	"sigmaos/apps/hotel"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v jobname cache", os.Args[0])
	}
	if err := hotel.RunWww(os.Args[1]); err != nil {
		db.DFatalf("Start %v err %v\n", os.Args[0], err)
	}
}
