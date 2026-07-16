package main

import (
	"os"

	"sigmaos/apps/hpsearch"
)

func main() {
	hpsearch.RunPruningTrainer(os.Args[1:])
}
