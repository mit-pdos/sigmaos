package main

import (
	"fmt"
	"os"

	"ulambda/group"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <grp>\n", os.Args[0])
		os.Exit(1)
	}
	group.RunMember(os.Args[1])
}
