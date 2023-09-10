package main

import (
	"os"
	"time"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
)

//
// Crashing proc
//

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(config.GetProcEnv())
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
