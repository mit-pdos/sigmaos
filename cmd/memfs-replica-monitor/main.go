package main

import (
	"log"
	"os"

	"ulambda/memfsd_replica"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("Usage: %v pid config-path union-dir-path", os.Args[0])
	}
	m := memfsd_replica.MakeMemfsReplicaMonitor(os.Args[1:])
	m.Work()
}
