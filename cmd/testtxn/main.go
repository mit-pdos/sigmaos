package main

import (
	"fmt"
	"os"

	"ulambda/twopc"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %v flwr index opcode\n", os.Args[0])
		os.Exit(1)
	}
	txn, err := twopc.MkTxnTest(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	txn.Run()
}
