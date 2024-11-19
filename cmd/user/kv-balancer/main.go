package main

import (
	"os"

	"sigmaos/apps/kv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 5 {
		db.DFatalf("Usage: %v <jobname> <kvdmcpu> <auto> <repl>", os.Args[0])
	}
	kv.RunBalancer(os.Args[1], os.Args[2], os.Args[3], os.Args[4])
}
