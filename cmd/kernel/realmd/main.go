package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	srv "sigmaos/realm/srv"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v usenetproxy", os.Args[0])
	}
	netproxy, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		db.DFatalf("Error parse netproxy: %v", err)
	}
	if err := srv.RunRealmSrv(netproxy); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
