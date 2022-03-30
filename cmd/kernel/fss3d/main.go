package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/s3"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("FATAL %v: incorrect number of args", os.Args[0])
	}
	fss3.RunFss3()
}
