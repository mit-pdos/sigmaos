package main

import (
	"fmt"
	"os"

	"ulambda/leadertest"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <dir> <last> <sec>\n", os.Args[0])
		os.Exit(1)
	}
	leadertest.RunLeader(os.Args[1], os.Args[2])
}
