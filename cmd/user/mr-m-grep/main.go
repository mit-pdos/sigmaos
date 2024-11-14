package main

import (
	"os"

	"sigmaos/grep"
	"sigmaos/mr"
)

func main() {
	mr.RunMapper(grep.Map, grep.Reduce, os.Args[1:])
}
