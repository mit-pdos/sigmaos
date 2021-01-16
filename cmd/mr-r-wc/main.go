package main

import (
	"fmt"
	"log"
	"os"

	"ulambda/mr"
	"ulambda/wc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v input...\n", os.Args[0])
		os.Exit(1)
	}
	m, err := mr.MakeReducer(wc.Reduce, os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	m.Work()
	log.Printf("Reducer: finished\n")
}
