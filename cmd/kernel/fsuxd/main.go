package main

import (
	"os"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/ux"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	os.Mkdir(np.UXEXPORT, 0755)
	fsux.RunFsUx(np.UXEXPORT)
}
