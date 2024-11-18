package main

import (
	"os"

	"sigmaos/apps/imgresize"
	db "sigmaos/debug"
)

func main() {
	w, err := imgresize.NewImgSrvRPC(os.Args[1:])
	if err != nil {
		db.DFatalf("%v", err)
	}
	w.Work()
}
