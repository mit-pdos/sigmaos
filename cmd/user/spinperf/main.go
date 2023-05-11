package main

import (
	"os"
	"strconv"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/microbenchmarks"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func main() {
	if len(os.Args) != 5 {
		db.DFatalf("Usage: %v sigmaproc sigma nthread niter id\nArgs: %v", os.Args[0], os.Args)
	}
	isSigmaProc, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		db.DFatalf("Error strconv: %v", err)
	}
	nthread, err := strconv.Atoi(os.Args[2])
	if err != nil {
		db.DFatalf("Error strconv: %v", err)
	}
	niter, err := strconv.Atoi(os.Args[3])
	if err != nil {
		db.DFatalf("Error strconv: %v", err)
	}
	id := os.Args[4]
	var sc *sigmaclnt.SigmaClnt
	if isSigmaProc {
		sc, err = sigmaclnt.MkSigmaClnt("spinperf-" + proc.GetPid().String())
		if err != nil {
			db.DFatalf("Error mkSigmaClnt: %v", err)
		}
		if err := sc.Started(); err != nil {
			db.DFatalf("Error started: %v", err)
		}
	}
	start := time.Now()
	spinPerf(nthread, niter)
	db.DPrintf(db.ALWAYS, "%v:  %v", id, time.Since(start))
	if isSigmaProc {
		sc.Exited(proc.MakeStatusInfo(proc.StatusOK, "elapsed time", time.Since(start)))
	}
}

func spinWorker(niter int, wg *sync.WaitGroup) {
	defer wg.Done()
	microbenchmarks.ConsumeCPU(niter)
}

func spinPerf(nthread, niter int) {
	var wg sync.WaitGroup
	wg.Add(nthread)
	for i := 0; i < nthread; i++ {
		go spinWorker(niter, &wg)
	}
	wg.Wait()
}
