package main

import (
	"os"
	"sigmaos/container"

	db "sigmaos/debug"
)

const ROOT = "/home/kaashoek/Downloads/rootfs"

func main() {
	if err := container.ExecContainer(ROOT); err != nil {
		db.DFatalf("%v: ExecContainer err %v\n", os.Args[0], err)
	}
	os.Exit(0)
}
