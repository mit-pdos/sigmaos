package main

import (
	"os"
	"strconv"

	"sigmaos/apps/hotel"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v jobname cache public", os.Args[0])
	}
	public, err := strconv.ParseBool(os.Args[3])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := hotel.RunWww(os.Args[1], public); err != nil {
		db.DFatalf("Start %v err %v\n", os.Args[0], err)
	}
}
