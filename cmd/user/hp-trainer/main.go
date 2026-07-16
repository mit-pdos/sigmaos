package main

import (
	"os"

	"sigmaos/apps/hpsearch"
)

func main() {
	hpsearch.RunTrainer(os.Args[1:])
}
