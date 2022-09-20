package main

import (
	"os"

	"sigmaos/grep"
	"sigmaos/mr"
)

func main() {
	mr.RunMapper(grep.Map, os.Args[1:])
}
