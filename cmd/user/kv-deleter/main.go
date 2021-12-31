package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "%v: <dir>\n", os.Args[0])
		os.Exit(1)
	}
	dl, err := kv.MakeDeleter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	dl.Delete(os.Args[1])
}
