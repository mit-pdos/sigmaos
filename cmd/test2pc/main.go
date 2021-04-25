package main

import (
	"fmt"
	"os"

	"ulambda/test2pc"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v index opcode\n", os.Args[0])
		os.Exit(1)
	}
	txn, err := test2pc.MkTest2pc(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}

	txn.Work()
}
