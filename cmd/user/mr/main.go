package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procinit"
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

	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true, procinit.PROCDEP: true})
	sclnt := procinit.MakeProcClnt(fsl, procinit.GetProcLayersMap())

	sclnt.Started(procinit.GetPid())

	// Start coordinators
	workers := map[string]bool{}
	for i := 0; i < ncoord; i++ {
		pid := proc.GenPid()
		a := proc.MakeProc(pid, "bin/user/mr-coord", os.Args[2:])
		sclnt.Spawn(a)
		workers[pid] = true
	}

	// Wait for coordinators to exit
	for w, _ := range workers {
		status, err := sclnt.WaitExit(w)
		if status != "OK" || err != nil {
			log.Printf("Wait %v failed %v %v\n", w, status, err)
		}
	}

	sclnt.Exited(procinit.GetPid(), "OK")
}
