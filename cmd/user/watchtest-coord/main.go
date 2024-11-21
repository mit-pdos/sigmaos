package main

import (
	"os"

	db "sigmaos/debug"
	drtest "sigmaos/fslib/dirreader/test"
	"sigmaos/proc"
	"sigmaos/util/perf"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v nworkers nfiles basedir\n", os.Args[0])
	}

	p, err := perf.NewPerf(proc.GetProcEnv(), "WATCH_TEST_COORD")
	if err != nil {
		db.DFatalf("%v: err %v", os.Args[0], err)
	}
	defer p.Done()

	c, err := drtest.NewTestCoord(os.Args[1:])
	if err != nil {
		db.DFatalf("%v: err %v", os.Args[0], err)
	}

	c.Run()
}
