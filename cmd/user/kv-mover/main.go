package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/kv"
)

func main() {
	if len(os.Args) != 5 {
		db.DFatalf("%v: <job> <epoch> <src> <dst>\n", os.Args[0])
	}
	mv, err := kv.MakeMover(os.Args[1], os.Args[2], os.Args[3], os.Args[4])
	if err != nil {
		db.DFatalf("Error MakeMover: %v", err)
	}
	mv.Move(os.Args[3], os.Args[4])
}
