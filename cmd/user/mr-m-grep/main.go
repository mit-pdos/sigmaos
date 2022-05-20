package main

import (
	"os"

	"ulambda/grep"
	"ulambda/mr"
)

func main() {
	mr.RunMapper(grep.Map, os.Args[1:])
}
