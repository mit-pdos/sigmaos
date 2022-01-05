package main

import (
	"fmt"
	"os"

	"ulambda/fslib"
	"ulambda/kv"
	"ulambda/proc"
)

func main() {
	if len(os.Args) < 1 {
		fmt.Fprintf(os.Stderr, "%v\n", os.Args[0])
		os.Exit(1)
	}
	clk := kv.MakeClerk("clerk-"+proc.GetPid(), fslib.Named())
	clk.Run()
}
