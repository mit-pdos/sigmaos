package main

import (
	"log"
	"os"

	"ulambda/ux"
)

func main() {
	if len(os.Args) != 1 {
		log.Fatalf("FATAL Usage: fsux")
	}
	fsux.RunFsUx("/tmp")
}
