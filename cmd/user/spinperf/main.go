package main

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func main() {
	if len(os.Args) != 7 {
		db.DFatalf("Usage: %v sigmaproc nthread niter id delay mem\nArgs: %v", os.Args[0], os.Args)
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
	d, err := time.ParseDuration(os.Args[5])
	if err != nil {
		db.DFatalf("Error ParseDuration: %v", err)
	}
	m, err := humanize.ParseBytes(os.Args[6])
	if err != nil {
		db.DFatalf("Error ParseBytes: %v", err)
	}
	time.Sleep(d)
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
	spinPerf(nthread, niter, m)
	db.DPrintf(db.ALWAYS, "%v:  %v (%v %v)", id, time.Since(start), d, m)
	if isSigmaProc {
		sc.Exited(proc.MakeStatusInfo(proc.StatusOK, "elapsed time", time.Since(start)))
	}
}

func spinWorker(niter int, m uint64, wg *sync.WaitGroup) {
	defer wg.Done()
	j := uint(0)
	n := uint64(1)
	mem := make([]byte, 1)
	if m > 0 {
		mem = make([]byte, m)
	}
	db.DPrintf(db.ALWAYS, "%v %v", niter, m)
	for i := uint(0); i < uint(niter); i++ {
		k := j * i
		j = k + i
		l := uint(len(mem))
		mem[j%l] = mem[k%l] + mem[i%l]
	}
	db.DPrintf(db.NEVER, "%v %v", j, n)
}

func spinPerf(nthread, niter int, m uint64) {
	var wg sync.WaitGroup
	wg.Add(nthread)
	for i := 0; i < nthread; i++ {
		go spinWorker(niter, m, &wg)
	}
	wg.Wait()
}
