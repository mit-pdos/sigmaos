package main

import (
	"time"

	"sigmaos/util/crash"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

//
// Crashing proc
//

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt err %v\n", err)
	}
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: err %v\n", err)
	}
	time.Sleep(1 * time.Millisecond)
	crash.Crash()
}
