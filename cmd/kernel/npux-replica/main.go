package main

import (
	"log"
	"os"

	"ulambda/replica"
)

func main() {
	if len(os.Args) < 6 {
		log.Fatalf("Usage: %v pid port config-path union-dir-path symlink-path", os.Args[0])
	}
	r := replica.MakeFsUxReplica(os.Args[1:])
	r.Work()
}
