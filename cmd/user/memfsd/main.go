package main

import (
	"log"

	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/proc"
)

func main() {
	linuxsched.ScanTopology()
	// started as a ulambda
	name := named.MEMFS + "/" + proc.GetPid()
	mfs, _, err := fslibsrv.MakeMemFs(name, name)
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	mfs.Serve()
	mfs.Done()
}
