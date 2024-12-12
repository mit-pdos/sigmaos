package main

import (
	"fmt"
	"os"

	"sigmaos/ft/leadertest"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <epoch> <dir>\n", os.Args[0])
		os.Exit(1)
	}
	leadertest.RunProc(os.Args[1], os.Args[2])
}
