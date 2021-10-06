package main

import (
	"fmt"
	"os"

	"ulambda/test_lambdas"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid sleep_length out <native>\n", os.Args[0])
		os.Exit(1)
	}
	l, err := test_lambdas.MakeSleeperl(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	l.Work()
	l.Exit()
}
