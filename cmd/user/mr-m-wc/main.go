package main

import (
	"os"

	"sigmaos/apps/mr"
	"sigmaos/apps/mr/wc"
)

func main() {
	mr.RunMapper(wc.Map, wc.Reduce, os.Args[1:])
}
