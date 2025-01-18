package main

import (
	"os"

	"sigmaos/apps/hotel"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 6 {
		db.DFatalf("Usage: %v jobname cache nindex maxRadius maxNumResults", os.Args[0])
	}
	db.DPrintf(db.ALWAYS, "SIGMA_DIALPROXY_FD: %v", os.Getenv("SIGMA_DIALPROXY_FD"))
	// if err := hotel.RunGeoSrv(os.Args[1], "", os.Args[3], os.Args[4], os.Args[5]); err != nil {
	// 	db.DFatalf("RunGeoSrv %v err %v\n", os.Args[0], err)
	// }
	if err := hotel.RunGeoSrv(os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]); err != nil {
		db.DFatalf("RunGeoSrv %v err %v\n", os.Args[0], err)
	}
}
