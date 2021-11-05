package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procinit"
)

//
// Runs a MR job.  Assumes directories for running job are setup.
//

func main() {
	if len(os.Args) != 6 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <ncoord> <nreducetasks> <mapper> <reducer> <crash>\n", os.Args[0])
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
		a := proc.MakeProc(pid, "bin/user/mr-coord", []string{os.Args[2], os.Args[3], os.Args[4], os.Args[5]})
		sclnt.Spawn(a)
		workers[pid] = true
	}

	// Wait for coordinators to exit
	for w, _ := range workers {
		status, err := sclnt.WaitExit(w)
		if err != nil && !strings.Contains(err.Error(), "file not found") || status != "OK" && status != "" {
			log.Fatalf("Wait failed %v, %v\n", err, status)
		}
	}

	sclnt.Exited(procinit.GetPid(), "OK")
}
