package main

import (
	"os"

	"sigmaos/apps/mr"
	"sigmaos/apps/mr/grep"
)

func main() {
	mr.RunReducer(grep.Reduce, os.Args[1:])
}
