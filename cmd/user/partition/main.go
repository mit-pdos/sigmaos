package main

import (
	"os"
	"time"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Partitioning proc
//

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(config.GetSigmaConfig())
	if err != nil {
		db.DFatalf("MkSigmaClnt: error %v\n", err)
	}
	sc.Started()
	if error := sc.Disconnect(sp.NAMED); error != nil {
		db.DFatalf("Disconnect %v name fails err %v\n", os.Args, error)
	}

	time.Sleep(100 * time.Millisecond)

	sc.ClntExitOK()
}
