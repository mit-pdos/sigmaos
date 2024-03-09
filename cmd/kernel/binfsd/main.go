// binsrv daemon. it takes as argument a local directory where to
// cache binaries.
package main

import (
	"os"

	"sigmaos/binsrv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("%s: Usage <dir>\n", os.Args[0])
	}
	if err := binsrv.RunBinFS(os.Args[1]); err != nil {
		db.DFatalf("RunBinFs %q err %v\n", os.Args[1], err)
	}
}
