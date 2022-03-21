package main

import (
	"fmt"
	"os"

	"ulambda/fenceclnttest"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <fence> <dir>\n", os.Args[0])
		os.Exit(1)
	}
	fenceclnttest.RunSecondary(os.Args[1], os.Args[2])
}
