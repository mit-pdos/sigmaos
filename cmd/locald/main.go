package main

import (
	"ulambda/locald"
)

func main() {
	ti, err := locald.ScanTopology()
	if err == nil {
		locald.PrintTopology(ti)
	}
}
