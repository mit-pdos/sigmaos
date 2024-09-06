package main

import (
	"os"

	"sigmaos/grep"
	"sigmaos/mr"
)

func main() {
	mr.RunMapper(grep.Map, nil, os.Args[1:])
}
