package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/kv"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v <crashhelper> <kvdncore> [auto]\n", os.Args[0])
	}
	kv.RunBalancer(os.Args[1], os.Args[2], os.Args[3])
}
