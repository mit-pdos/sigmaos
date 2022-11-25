package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/procd"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v realmbin coreIv spawningSys", os.Args[0])
	}
	spawningSys, err := strconv.ParseBool(os.Args[3])
	if err != nil {
		db.DFatalf("Err start procd %v", err)
	}
	procd.RunProcd(os.Args[1], os.Args[2], spawningSys)
}
