package main

import (
	"os"

	"sigmaos/apps/hotel"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 3 && len(os.Args) != 4 && len(os.Args) != 5 {
		db.DFatalf("Usage: %v jobname cache", os.Args[0])
	}
	if err := hotel.RunWww(os.Args[1], len(os.Args) >= 4 && os.Args[3] == "only-www", len(os.Args) == 5 && os.Args[4] == "running-in-docker"); err != nil {
		db.DFatalf("Start %v err %v\n", os.Args[0], err)
	}
}
