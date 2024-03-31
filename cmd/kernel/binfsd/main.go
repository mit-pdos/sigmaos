// binsrv daemon. it takes as argument a local directory where to
// cache binaries.
package main

import (
	"os"

	"sigmaos/binsrv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 3 {
		db.DFatalf("%s: Usage <kernelid> <uprocpid>\n", os.Args[0])
	}
	if err := binsrv.RunBinFS(os.Args[1], os.Args[2]); err != nil {
		db.DFatalf("RunBinFs %v err %v\n", os.Args, err)
	}
}
