package main

import (
	"fmt"
	"os"

	"ulambda/leadertest"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <fence> <dir> <last> <sec>\n", os.Args[0])
		os.Exit(1)
	}
	leadertest.RunLeader(os.Args[1], os.Args[2], os.Args[3])
}
