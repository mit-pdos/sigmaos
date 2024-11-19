package main

import (
	"os"

	"sigmaos/besched/srv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	srv.Run()
}
