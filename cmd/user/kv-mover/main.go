package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "%v: <epoch> <src> <dst>\n", os.Args[0])
		os.Exit(1)
	}
	mv, err := kv.MakeMover(os.Args[1], os.Args[2], os.Args[3])
	if err == nil {
		mv.Move(os.Args[2], os.Args[3])
	} else {
		os.Exit(1)
	}
}
