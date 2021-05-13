package main

import (
	"log"
	"os"

	"ulambda/replica"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("Usage: %v pid config-path union-dir-path", os.Args[0])
	}
	m := replica.MakeReplicaMonitor(os.Args[1:])
	m.Work()
}
