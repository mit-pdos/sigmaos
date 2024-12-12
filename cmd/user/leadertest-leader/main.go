package main

import (
	"fmt"
	"os"

	"sigmaos/ft/leadertest"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <dir> <last> <child>\n", os.Args[0])
		os.Exit(1)
	}
	leadertest.RunLeader(os.Args[1], os.Args[2], os.Args[3])
}
