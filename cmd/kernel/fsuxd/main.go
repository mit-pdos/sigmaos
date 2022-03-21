package main

import (
	"log"
	"os"

	np "ulambda/ninep"
	"ulambda/ux"
)

func main() {
	if len(os.Args) != 1 {
		log.Fatalf("FATAL Usage: fsux")
	}
	os.Mkdir(np.UXEXPORT, 0755)
	fsux.RunFsUx(np.UXEXPORT)
}
