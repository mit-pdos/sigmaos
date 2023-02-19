package main

import (
	"os"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Crashing proc
//

func main() {
	sc, err := sigmaclnt.MkSigmaClnt(os.Args[0] + "-" + proc.GetPid().String())
	if err != nil {
		db.DFatalf("MkSigmaClnt: error %v\n", err)
	}
	if error := sc.Disconnect(sp.NAMED); error != nil {
		db.DFatalf("Disconnect %v name fails err %v\n", os.Args, error)
	}

	time.Sleep(100 * time.Millisecond)

	sc.Exited(proc.MakeStatus(proc.StatusOK))
}
