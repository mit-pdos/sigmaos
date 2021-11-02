package main

import (
	"log"

	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/procinit"
)

func main() {
	linuxsched.ScanTopology()
	name := named.MEMFS + "/" + procinit.GetPid()
	mfs, err := fslibsrv.StartMemFs(name, name)
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	sclnt := procinit.MakeProcClnt(mfs.FsLib, procinit.GetProcLayersMap())
	sclnt.Started(procinit.GetPid())

	mfs.FsServer.GetStats().MakeElastic(mfs.FsLib, procinit.GetPid())
	mfs.Wait()
	mfs.FsServer.GetStats().Done()

	sclnt.Exited(procinit.GetPid(), "OK")
	mfs.FsLib.ShutdownFs(name)
}
