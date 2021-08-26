package main

import (
	"log"
	"os"

	"ulambda/idemproc"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %v pid", os.Args[0])
	}

	m := idemproc.MakeMonitor(os.Args[1:])
	m.Work()
	m.Exit()
}
