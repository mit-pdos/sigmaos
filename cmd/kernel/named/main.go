package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/namesrv"
)

func main() {
	if err := namesrv.Run(os.Args); err != nil {
		db.DFatalf("%v: err %v\n", os.Args[0], err)
	}
}
