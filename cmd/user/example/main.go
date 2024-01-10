package main

import (
	"log"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt: error %v\n", err)
	}
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}

	log.Printf("example program ran successfully\n")

	sc.ClntExit(proc.NewStatusInfo(proc.StatusOK, "Hello world", []string{}))
}
