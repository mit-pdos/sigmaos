package main

import (
	"log"
	"os"

	"ulambda/replica"
)

func main() {
	if len(os.Args) < 5 {
		log.Fatalf("Usage: %v pid port config-path union-dir-path", os.Args[0])
	}
	r := replica.MakeNpUxReplica(os.Args[1:])
	r.Work()
}
