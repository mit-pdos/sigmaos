package main

import (
	"fmt"
	"os"

	"ulambda/perf"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v MB\n", os.Args[0])
		os.Exit(1)
	}
	t, err := perf.MakeMemfsBandwidthTest(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	t.Work()
}
