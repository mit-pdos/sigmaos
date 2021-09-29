package main

import (
	"fmt"
	"os"

	"ulambda/benchmarks"
)

func main() {
	if len(os.Args) < 6 {
		fmt.Fprintf(os.Stderr, "Usage: %v spawnsPerSecond seconds ninep/native dim its \n", os.Args[0])
		os.Exit(1)
	}
	r, err := benchmarks.MakeRival(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	r.Work()
}
