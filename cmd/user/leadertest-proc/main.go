package main

import (
	"fmt"
	"os"

	"ulambda/leadertest"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <epoch> <dir> <N>\n", os.Args[0])
		os.Exit(1)
	}
	leadertest.RunProc(os.Args[1], os.Args[2], os.Args[3])
}
