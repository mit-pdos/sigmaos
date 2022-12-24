package main

import (
	"os"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

//
// Crashing proc
//

func main() {
	fsl, err := fslib.MakeFsLib(os.Args[0] + "-" + proc.GetPid().String())
	if err != nil {
		db.DFatalf("MakeFsLib err %v\n", err)
	}
	pclnt := procclnt.MakeProcClnt(fsl)
	err = pclnt.Started()
	if err != nil {
		db.DFatalf("Started: err %v\n", err)
	}
	time.Sleep(1 * time.Millisecond)
	os.Exit(2)
}
