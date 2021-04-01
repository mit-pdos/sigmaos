package main

import (
	"fmt"
	"os"

	"ulambda/perf"
)

func main() {
	if len(os.Args) < 5 {
		fmt.Fprintf(os.Stderr, "Usage: %v nSpinners dim iterations <native/9p>\n", os.Args[0])
		os.Exit(1)
	}
	s, err := perf.MakeSpinTestStarter(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	s.Work()
}
