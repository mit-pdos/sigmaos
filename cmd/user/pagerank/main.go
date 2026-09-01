package main

import (
	"os"

	"sigmaos/apps/pagerank"
)

func main() {
	pagerank.Pagerank(os.Args[1:])
}
