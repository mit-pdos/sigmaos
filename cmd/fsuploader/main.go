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
	up, err := fslambda.MakeUploader(os.Args[1:], true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	up.Work()
	up.Exit()
}
