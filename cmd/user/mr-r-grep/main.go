package main

import (
	"os"

	"ulambda/grep"
	"ulambda/mr"
)

func main() {
	mr.RunReducer(grep.Reduce, os.Args[1:])
}
