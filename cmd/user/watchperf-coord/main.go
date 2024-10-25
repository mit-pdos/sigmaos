package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/watch"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v nworkers nstartfiles ntrials basedir\n", os.Args[0])
	}

	c, err := watch.NewCoord(os.Args[1:])
	if err != nil {
		db.DFatalf("%v: err %v", os.Args[0], err)
	}

	c.Run()
}
