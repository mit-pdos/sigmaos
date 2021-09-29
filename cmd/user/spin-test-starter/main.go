package main

import (
	"fmt"
	"os"

	"ulambda/benchmarks"
)

func main() {
	if len(os.Args) < 6 {
		fmt.Fprintf(os.Stderr, "Usage: %v nSpinners dim iterations <native/9p/remote/baseline> <remote/local>\n", os.Args[0])
		os.Exit(1)
	}
	s, err := benchmarks.MakeSpinTestStarter(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	s.Work()
}
