package main

import (
	"os"

	"sigmaos/mr"
	"sigmaos/wc"
)

func main() {
	mr.RunMapper(wc.Map, os.Args[1:])
}
