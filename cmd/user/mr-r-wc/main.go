package main

import (
	"os"

	"sigmaos/apps/mr"
	"sigmaos/apps/mr/wc"
)

func main() {
	mr.RunReducer(wc.Reduce, os.Args[1:])
}
