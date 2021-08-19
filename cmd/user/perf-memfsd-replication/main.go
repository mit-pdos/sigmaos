package main

import (
	"fmt"
	"os"

	"ulambda/perf"
)

func main() {
	if len(os.Args) < 5 {
		fmt.Fprintf(os.Stderr, "Usage: %v n_clients secs replicated n-servers\n", os.Args[0])
		os.Exit(1)
	}
	t := perf.MakeMemfsReplicationTest(os.Args[1:])
	t.Work()
}
