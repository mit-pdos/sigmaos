package main

import (
	"log"
	"os"

	"ulambda/s3"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("%v: incorrect number of args", os.Args[0])
	}
	fss3 := fss3.MakeFss3(os.Args[1])
	fss3.Serve()
}
