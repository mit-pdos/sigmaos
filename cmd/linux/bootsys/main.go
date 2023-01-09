package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/system"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v realmid", os.Args[0])
	}
	_, err := system.Boot(os.Args[1], "bootkernelclnt")
	if err != nil {
		db.DFatalf("%v: Boot %v\n", err, os.Args[0])
	}
	for {
	}
}
