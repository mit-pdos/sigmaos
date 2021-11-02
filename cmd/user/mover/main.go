package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	mv, err := kv.MakeMover(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	mv.Work()
	mv.Exit()
}
