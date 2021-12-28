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
	// started as a ulambda
	name := np.MEMFS + "/" + proc.GetPid()
	mfs, _, err := fslibsrv.MakeMemFs(name, name)
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	mfs.Serve()
	mfs.Done()
}
