package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v opcode pids\n", os.Args[0])
		os.Exit(1)
	}
	bl, err := kv.MakeBalancer(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	bl.Balance()
	bl.Exit()
	os.Exit(0)
}
