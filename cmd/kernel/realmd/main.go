package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	srv "sigmaos/realm/srv"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v usedialproxy", os.Args[0])
	}
	dialproxy, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		db.DFatalf("Error parse dialproxy: %v", err)
	}
	if err := srv.RunRealmSrv(dialproxy); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
