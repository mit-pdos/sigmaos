package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/mr"
)

func main() {
	w, err := mr.NewCoord(os.Args[1:])
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	w.Work()
}
