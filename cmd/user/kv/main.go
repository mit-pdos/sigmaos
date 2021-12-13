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
	name := named.MEMFS + "/" + proc.GetPid()
	mfs, _, err := fslibsrv.MakeMemFs(name, name)
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}

	mfs.FsServer.GetStats().MakeElastic(mfs.FsLib, proc.GetPid())
	mfs.Serve()
	mfs.Done()
	mfs.FsServer.GetStats().Done()
}
