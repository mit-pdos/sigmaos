package main

import (
	"fmt"
	"os"

	"sigmaos/www"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <tree>\n", os.Args[0])
		os.Exit(1)
	}
	www.RunWwwd(os.Args[1])
}
