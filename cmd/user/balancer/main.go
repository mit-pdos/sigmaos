package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v [auto] [crash]\n", os.Args[0])
		os.Exit(1)
	}
	kv.RunBalancer(os.Args[1], os.Args[2])
}
