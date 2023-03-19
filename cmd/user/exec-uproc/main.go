package main

import (
	"os"
	"sigmaos/container"

	db "sigmaos/debug"
)

func main() {
	if err := container.ExecUProc(); err != nil {
		db.DFatalf("%v: ExecUProc err %v\n", os.Args[0], err)
	}
	os.Exit(0)
}
