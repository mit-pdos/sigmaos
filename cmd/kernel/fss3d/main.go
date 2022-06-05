package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/s3"
)

func main() {
	if len(os.Args) < 1 {
		db.DFatalf("Usage: %v [buckets]", os.Args[0])
	}
	fss3.RunFss3(os.Args[1:])
}
