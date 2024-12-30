// A proc to test partition for [procclnt_test]
package main

import (
	"os"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt: error %v", err)
	}
	sc.Started()
	_, err = sc.GetDir(sp.NAMED)
	if err != nil {
		db.DFatalf("Named GetDir error: %v", err)
	}
	if error := sc.Disconnect(sp.NAMED); error != nil {
		db.DFatalf("Disconnect %v name fails err %v", os.Args, error)
	}

	time.Sleep(100 * time.Millisecond)

	sc.ClntExitOK()
}
