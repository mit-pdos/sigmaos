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
	mfs, err := fslibsrv.StartMemFs(name, name)
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}

	mfs.FsServer.GetStats().MakeElastic(mfs.FsLib, proc.GetPid())
	mfs.Wait()
	mfs.FsServer.GetStats().Done()
}
