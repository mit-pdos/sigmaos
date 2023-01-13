package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/procd"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v realmbin spawningSys", os.Args[0])
	}
	spawningSys, err := strconv.ParseBool(os.Args[2])
	if err != nil {
		db.DFatalf("Err start procd %v", err)
	}
	procd.RunProcd(os.Args[1], spawningSys)
}
