package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/imgresizesrv"
)

func main() {
	w, err := imgresizesrv.NewImgSrvRPC(os.Args[1:])
	if err != nil {
		db.DFatalf("%v", err)
	}
	w.Work()
}
