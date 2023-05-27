package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/namedv1"
)

// Usage: <named>

func main() {
	if err := namedv1.Run(os.Args); err != nil {
		db.DFatalf("%v: err %v\n", os.Args[0], err)
	}
}
