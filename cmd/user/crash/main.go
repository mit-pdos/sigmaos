package main

import (
	"os"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

//
// Crashing proc
//

func main() {
	sc, err := sigmaclnt.MkSigmaClnt(os.Args[0] + "-" + proc.GetPid().String())
	if err != nil {
		db.DFatalf("MkSigmaClnt err %v\n", err)
	}
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: err %v\n", err)
	}
	time.Sleep(1 * time.Millisecond)
	os.Exit(2)
}
