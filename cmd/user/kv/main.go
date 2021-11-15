package main

import (
	"log"

	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/procclnt"
)

func main() {
	linuxsched.ScanTopology()
	name := named.MEMFS + "/" + proc.GetPid()
	mfs, err := fslibsrv.StartMemFs(name, name)
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	sclnt := procclnt.MakeProcClnt(mfs.FsLib)
	sclnt.Started(proc.GetPid())

	mfs.FsServer.GetStats().MakeElastic(mfs.FsLib, proc.GetPid())
	mfs.Wait()
	mfs.FsServer.GetStats().Done()

	sclnt.Exited(proc.GetPid(), "OK")
	mfs.FsLib.ShutdownFs(name)
}
