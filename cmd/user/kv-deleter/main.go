package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "%v: <dir> <crash>\n", os.Args[0])
		os.Exit(1)
	}
	dl, err := kv.MakeDeleter(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	dl.Delete(os.Args[1])
}
