package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/imgresize"
)

func main() {
	w, err := imgresize.NewImgSrvRPC(os.Args[1:])
	if err != nil {
		db.DFatalf("%v", err)
	}
	w.Work()
}
