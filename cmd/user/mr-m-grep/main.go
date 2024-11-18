package main

import (
	"os"

	"sigmaos/apps/mr"
	"sigmaos/apps/mr/grep"
)

func main() {
	mr.RunMapper(grep.Map, grep.Reduce, os.Args[1:])
}
