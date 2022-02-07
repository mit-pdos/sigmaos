package main

import (
	"log"

	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/proc"
)

func main() {
	linuxsched.ScanTopology()
	name := np.MEMFS + "/" + proc.GetPid()
	mfs, _, err := fslibsrv.MakeMemFs(name, name)
	if err != nil {
		log.Fatalf("FATAL MakeMemFs %v\n", err)
	}
	mfs.Serve()
	mfs.Done()
}
