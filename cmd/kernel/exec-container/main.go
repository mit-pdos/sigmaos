package main

import (
	"os"
	"sigmaos/container"

	db "sigmaos/debug"
)

func main() {
	if err := container.ExecContainer(); err != nil {
		db.DFatalf("%v: ExecContainer err %v\n", os.Args[0], err)
	}
	os.Exit(0)
}
