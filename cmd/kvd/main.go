package main

import (
	"fmt"
	"os"

	"ulambda/kvlambda"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid args...\n", os.Args[0])
		os.Exit(1)
	}
	kv, err := kvlambda.MakeKv(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	kv.Work()
	kv.Exit()
}
