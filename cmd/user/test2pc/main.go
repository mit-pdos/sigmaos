package main

import (
	"fmt"
	"os"

	"ulambda/test2pc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v index opcode\n", os.Args[0])
		os.Exit(1)
	}
	p, err := test2pc.MkTest2Participant(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	p.Work()
}
