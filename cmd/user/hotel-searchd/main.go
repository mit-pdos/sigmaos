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
	hotel.RunSearchSrv(os.Args[0])
}
