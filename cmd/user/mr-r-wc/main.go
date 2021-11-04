package main

import (
	"fmt"
	"os"

	"ulambda/mr"
	"ulambda/wc"
)

func main() {
	m, err := mr.MakeReducer(wc.Reduce, os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	m.Work()
	m.Exit()
}
