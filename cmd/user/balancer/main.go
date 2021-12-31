package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v [auto]\n", os.Args[0])
		os.Exit(1)
	}
	kv.RunBalancer(os.Args[1])
}
