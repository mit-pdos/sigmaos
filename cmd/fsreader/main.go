package main

import (
	"fmt"
	"os"

	"ulambda/fslambda"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid input...\n", os.Args[0])
		os.Exit(1)
	}
	m, err := fslambda.MakeReader(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	m.Work()
}
