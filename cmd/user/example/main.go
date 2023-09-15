package main

import (
	"log"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
)

func main() {
	sc, err := sigmaclnt.MkSigmaClnt("example")
	if err != nil {
		db.DFatalf("MkSigmaClnt: error %v\n", err)
	}
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}

	log.Printf("Hello world\n")

	sc.ClntExitOK()
}
