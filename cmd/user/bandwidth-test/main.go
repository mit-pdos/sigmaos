package main

import (
	"fmt"
	"os"

	"ulambda/benchmarks"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v MB <s3/memfs>\n", os.Args[0])
		os.Exit(1)
	}
	t, err := benchmarks.MakeBandwidthTest(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	t.Work()
}
