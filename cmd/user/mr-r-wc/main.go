package main

import (
	"os"

	"sigmaos/mr"
	"sigmaos/wc"
)

func main() {
	mr.RunReducer(wc.Reduce, os.Args[1:])
}
