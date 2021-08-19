package main

import (
	"fmt"
	"os"

	"ulambda/kv"
)

func main() {
	mo, err := kv.MakeMonitor(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	mo.Work()
	mo.Exit()
	os.Exit(0)
}
