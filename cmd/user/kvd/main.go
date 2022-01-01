package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <grp>\n", os.Args[0])
		os.Exit(1)
	}
	kv.RunKv(os.Args[1])
}
