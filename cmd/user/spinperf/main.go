package main

import (
	"os"
	"strconv"
	"sync"
	"time"

	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v nthread niter id", os.Args[0])
	}
	nthread, err := strconv.Atoi(os.Args[1])
	if err != nil {
		db.DFatalf("Error strconv: %v", err)
	}
	niter, err := strconv.Atoi(os.Args[2])
	if err != nil {
		db.DFatalf("Error strconv: %v", err)
	}
	id := os.Args[3]
	start := time.Now()
	spinPerf(nthread, niter)
	db.DPrintf(db.ALWAYS, "%v:  %v", id, time.Since(start))
}

func spinWorker(niter int, wg *sync.WaitGroup) {
	defer wg.Done()
	j := 0
	for i := 0; i < niter; i++ {
		j = j*i + i
	}
	db.DPrintf(db.NEVER, "%v", j)
}

func spinPerf(nthread, niter int) {
	var wg sync.WaitGroup
	wg.Add(nthread)
	for i := 0; i < nthread; i++ {
		go spinWorker(niter, &wg)
	}
	wg.Wait()
}
