package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// Runs a MR job.  Assumes directories for running job are setup.
//

func main() {
	if len(os.Args) != 7 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <ncoord> <nreducetasks> <mapper> <reducer> <crash-task><crash-coord>\n", os.Args[0])
		os.Exit(1)
	}

	ncoord, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ncoord %v is not a number\n", os.Args[1])
		os.Exit(1)
	}

	fsl := fslib.MakeFsLib("mr-wc")

	sclnt := procclnt.MakeProcClnt(fsl)

	sclnt.Started(proc.GetPid())

	// Start coordinators
	coords := map[string]bool{}
	for i := 0; i < ncoord; i++ {
		if i == ncoord-1 {
			// last coordinator doesn't crash
			os.Args[len(os.Args)-1] = "NO"
		}
		a := proc.MakeProc("bin/user/mr-coord", os.Args[2:])
		sclnt.Spawn(a)
		coords[a.Pid] = true
	}

	// Wait for coordinators to exit
	for c, _ := range coords {
		status, err := sclnt.WaitExit(c)
		if status != "OK" || err != nil {
			log.Printf("Wait %v failed %v %v\n", c, status, err)
		}
	}

	sclnt.Exited(proc.GetPid(), "OK")
}
