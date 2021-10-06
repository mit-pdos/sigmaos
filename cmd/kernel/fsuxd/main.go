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
	fsux := fsux.MakeFsUx("/tmp", os.Args[1])
	fsux.Serve()
}
