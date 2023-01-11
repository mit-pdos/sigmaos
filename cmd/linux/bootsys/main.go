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
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v <nmachine>", os.Args[0])
	}
	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("%s: Atoi err %v\n", os.Args[0], err)
	}
	_, err := system.Boot(n, "bootkernelclnt")
	if err != nil {
		log.Fatalf("%v: Boot %v\n", err, os.Args[0])
	}
	for {
		time.Sleep(100 * time.Second)
	}
}
