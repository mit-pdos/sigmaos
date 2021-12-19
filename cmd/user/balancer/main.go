package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %v\n", os.Args[0])
		os.Exit(1)
	}
	kv.RunBalancer()
}
