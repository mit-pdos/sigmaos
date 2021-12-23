package main

import (
	"os"

	"ulambda/mr"
	"ulambda/wc"
)

func main() {
	mr.RunMapper(wc.Map, os.Args[1:])
}
