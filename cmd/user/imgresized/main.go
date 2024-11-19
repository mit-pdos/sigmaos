package main

import (
	"fmt"
	"os"

	"sigmaos/apps/imgresize"
)

func main() {
	w, err := imgresize.NewImgSrv(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	w.Work()
}
