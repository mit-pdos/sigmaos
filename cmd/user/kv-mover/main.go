package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "%v: <src> <dst> <crash>\n", os.Args[0])
		os.Exit(1)
	}
	mv, err := kv.MakeMover(os.Args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	mv.Move(os.Args[1], os.Args[2])
}
