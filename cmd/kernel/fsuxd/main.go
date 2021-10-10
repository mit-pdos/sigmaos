package main

import (
	"log"
	"os"

	"ulambda/ux"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: fsux <pid>")
	}
	fsux.RunFsUx("/tmp", os.Args[1])
}
