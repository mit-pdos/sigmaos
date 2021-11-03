package main

import (
	"fmt"
	"os"

	"ulambda/mr"
)

func main() {
	w, err := mr.MakeWorker(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	w.Work()
}
