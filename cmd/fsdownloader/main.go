package main

import (
	"fmt"
	"os"

	"ulambda/fslambda"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid src dest\n", os.Args[0])
		os.Exit(1)
	}
	down, err := fslambda.MakeDownloader(os.Args[1:], false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	down.Work()
	down.Exit()
}
