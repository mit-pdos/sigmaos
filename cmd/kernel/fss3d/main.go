package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/proxy/s3"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	fss3.RunFss3()
}
