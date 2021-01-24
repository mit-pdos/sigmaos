package main

import (
	"fmt"
	"os"

	"ulambda/kvlambda"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid\n", os.Args[0])
		os.Exit(1)
	}
	sh, err := kvlambda.MakeSharder(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	sh.Work()
	sh.Exit()
}
