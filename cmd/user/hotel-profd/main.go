package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/hotel"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v jobname public cache", os.Args[0])
	}
	public, err := strconv.ParseBool(os.Args[2])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := hotel.RunProfSrv(os.Args[1], public, os.Args[3]); err != nil {
		db.DFatalf("Start %v err %v\n", os.Args[0], err)
	}
}
