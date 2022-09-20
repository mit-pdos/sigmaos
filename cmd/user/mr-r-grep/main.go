package main

import (
	"os"

	"sigmaos/grep"
	"sigmaos/mr"
)

func main() {
	mr.RunReducer(grep.Reduce, os.Args[1:])
}
