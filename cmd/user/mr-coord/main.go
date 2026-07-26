package main

import (
	"os"

	"sigmaos/apps/mr/coord"
	db "sigmaos/debug"
)

func main() {
	w, err := coord.NewCoord(os.Args[1:])
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	w.Work()
}
