package main

import (
	"os"

	db "sigmaos/debug"
	drtest "sigmaos/fslib/dirreader/test"
	"sigmaos/perf"
	"sigmaos/proc"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v id nfiles workdir readydir\n", os.Args[0])
	}

	p, err := perf.NewPerf(proc.GetProcEnv(), "WATCH_TEST_WORKER")
	if err != nil {
		db.DFatalf("%v: err %v", os.Args[0], err)
	}
	defer p.Done()


	w, err := drtest.NewTestWorker(os.Args[1:])
	if err != nil {
		db.DFatalf("%v: err %v", os.Args[0], err)
	}

	w.Run()
}
