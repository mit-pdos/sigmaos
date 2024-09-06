package main

import (
	"os"

	"sigmaos/mr"
	"sigmaos/wc"
)

func main() {
	mr.RunMapper(wc.Map, wc.Reduce, os.Args[1:])
}
