package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/proxy/s3/srv"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	srv.RunFss3()
}
