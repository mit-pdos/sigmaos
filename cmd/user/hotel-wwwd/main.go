package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/hotel"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	if err := hotel.RunWww(os.Args[0]); err != nil {
		db.DFatalf("Start %v err %v\n", os.Args[0], err)
	}
}
