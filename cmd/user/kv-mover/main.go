package main

import (
	"os"

	"sigmaos/apps/kv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 7 {
		db.DFatalf("%v: <job> <epoch> <shard> <src> <dst> <repl>\n", os.Args[0])
	}
	mv, err := kv.NewMover(os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6])
	if err != nil {
		db.DFatalf("Error NewMover: %v", err)
	}
	mv.Move(os.Args[4], os.Args[5])
}
