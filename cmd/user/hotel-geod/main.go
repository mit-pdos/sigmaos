package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/hotel"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v jobname cache nindex", os.Args[0])
	}
	if err := hotel.RunGeoSrv(os.Args[1], os.Args[3]); err != nil {
		db.DFatalf("RunGeoSrv %v err %v\n", os.Args[0], err)
	}
}
