package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/hotel"
)

func main() {
	if len(os.Args) != 6 {
		db.DFatalf("Usage: %v jobname cache nindex maxRadius maxNumResults", os.Args[0])
	}
	if err := hotel.RunGeoSrv(os.Args[1], os.Args[3], os.Args[4], os.Args[5]); err != nil {
		db.DFatalf("RunGeoSrv %v err %v\n", os.Args[0], err)
	}
}
