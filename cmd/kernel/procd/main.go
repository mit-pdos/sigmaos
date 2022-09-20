package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/procd"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v realmbin coreIv", os.Args[0])
	}
	procd.RunProcd(os.Args[1], os.Args[2])
}
