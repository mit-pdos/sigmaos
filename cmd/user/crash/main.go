package main

import (
	"log"
	"os"

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
	err := pclnt.Started(proc.GetPid())
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}
	os.Exit(2)
}
