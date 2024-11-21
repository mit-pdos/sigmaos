package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/sched/lcsched/srv"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	srv.Run()
}
