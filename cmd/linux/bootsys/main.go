package main

import (
	"log"
	"os"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/system"
)

func main() {
	if len(os.Args) < 3 {
		db.DFatalf("Usage: %v <realmid> <nmachine>", os.Args[0])
	}
	n, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatalf("%s: Atoi err %v", os.Args[0], err)
	}
	_, err = system.Boot(os.Args[1], n, "bootkernelclnt")
	if err != nil {
		log.Fatalf("%v: Boot %v", os.Args[0], err)
	}
	for {
		time.Sleep(100 * time.Second)
	}
}
