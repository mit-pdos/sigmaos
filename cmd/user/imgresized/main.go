package main

import (
	"fmt"
	"os"

	"sigmaos/imgresized"
)

func main() {
	w, err := imgresized.MakeImgd(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	w.Work()
}
