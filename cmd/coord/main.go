package main

import (
	"fmt"
	"log"
	"os"

	"ulambda/twopc"
)

func main() {
	log.Printf("coord\n")
	if len(os.Args) < 5 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid opcode txnprog flwrs\n",
			os.Args[0])
		os.Exit(1)
	}
	cd, err := twopc.MakeCoord(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	cd.TwoPC()
}
