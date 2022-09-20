package main

import (
	"fmt"
	"os"

	"sigmaos/group"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <jobdir> <grp>\n", os.Args[0])
		os.Exit(1)
	}
	group.RunMember(os.Args[1], os.Args[2])
}
