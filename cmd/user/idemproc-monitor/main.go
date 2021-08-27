package main

import (
	"log"
	"os"

	"ulambda/monitor"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %v pid", os.Args[0])
	}

	m := monitor.MakeMonitor(os.Args[1:])
	m.Work()
	m.Exit()
}
