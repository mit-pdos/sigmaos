package main

import (
	"fmt"
	"os"

	"ulambda/perf"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v spawnsPerSecond seconds\n", os.Args[0])
		os.Exit(1)
	}
	r, err := perf.MakeRival(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	r.Work()
}
