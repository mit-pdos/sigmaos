package main

import (
	"fmt"
	"os"

	"sigmaos/apps/www"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <job> <tree>\n", os.Args[0])
		os.Exit(1)
	}
	www.RunWwwd(os.Args[1], os.Args[2])
}
