package main

import (
	"log"
	"os"

	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/procinit"
)

func main() {
	linuxsched.ScanTopology()
	name := named.MEMFS + "/" + os.Args[1]
	mfs, err := fslibsrv.StartMemFs(name, name)
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	sclnt := procinit.MakeProcClnt(mfs.FsLib, procinit.GetProcLayersMap())
	sclnt.Started(os.Args[1])

	mfs.FsServer.GetStats().MakeElastic(mfs.FsLib, os.Args[1])
	mfs.Wait()
	mfs.FsServer.GetStats().Done()

	sclnt.Exited(os.Args[1], "OK")
	mfs.FsLib.ShutdownFs(name)
}
