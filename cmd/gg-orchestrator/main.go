package main

import (
	"fmt"
	"os"

	"ulambda/gg"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid targets\n", os.Args[0])
		os.Exit(1)
	}
	orc, err := gg.MakeOrchestrator(os.Args[1:], true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	orc.Work()
	orc.Exit()
}
