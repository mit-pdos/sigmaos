package main

import (
	"fmt"
	"os"

	imgsrv "sigmaos/apps/imgresize/srv"
)

func main() {
	w, err := imgsrv.NewImgSrv(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	w.Work()
}
