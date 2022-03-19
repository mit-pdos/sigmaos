package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "%v: <epoch> <dir>\n", os.Args[0])
		os.Exit(1)
	}
	dl, err := kv.MakeDeleter(os.Args[1], os.Args[2])
	if err == nil {
		dl.Delete(os.Args[2])
	} else {
		os.Exit(1)
	}
}
