package main

import (
	"log"
	"os"
	"time"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// Crashing proc
//

func main() {
	fsl := fslib.MakeFsLib(os.Args[0] + "-" + proc.GetPid().String())
	pclnt := procclnt.MakeProcClnt(fsl)
	err := pclnt.Started()
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}
	time.Sleep(500 * time.Millisecond)
	os.Exit(2)
}
