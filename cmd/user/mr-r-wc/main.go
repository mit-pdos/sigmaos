package main

import (
	"os"

	"ulambda/mr"
	"ulambda/wc"
)

func main() {
	mr.RunReducer(wc.Reduce, os.Args[1:])
}
