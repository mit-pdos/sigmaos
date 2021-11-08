package main

import (
	"log"

	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/procinit"
)

func main() {
	linuxsched.ScanTopology()
	name := named.MEMFS + "/" + proc.GetPid()
	mfs, err := fslibsrv.StartMemFs(name, name)
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	sclnt := procinit.MakeProcClnt(mfs.FsLib, procinit.GetProcLayersMap())
	sclnt.Started(proc.GetPid())

	mfs.FsServer.GetStats().MakeElastic(mfs.FsLib, proc.GetPid())
	mfs.Wait()
	mfs.FsServer.GetStats().Done()

	sclnt.Exited(proc.GetPid(), "OK")
	mfs.FsLib.ShutdownFs(name)
}
