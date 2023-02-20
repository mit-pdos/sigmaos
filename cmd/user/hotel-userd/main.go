package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/hotel"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v jobname public", os.Args[0])
	}
	public, err := strconv.ParseBool(os.Args[2])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := hotel.RunUserSrv(os.Args[0], public); err != nil {
		db.DFatalf("Start %v err %v\n", os.Args[0], err)
	}
}
