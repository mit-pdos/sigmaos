package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/watch"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v id ntrials watchdir responsedir tempdir\n", os.Args[0])
	}

	w, err := watch.NewWorker(os.Args[1:])
	if err != nil {
		db.DFatalf("%v: err %v", os.Args[0], err)
	}

	w.Run()
}

