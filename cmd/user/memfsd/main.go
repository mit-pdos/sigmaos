package main

import (
	"log"
	"os"

	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/procinit"
	"ulambda/seccomp"
)

func main() {
	linuxsched.ScanTopology()
	// started as a ulambda
	name := named.MEMFS + "/" + os.Args[1]
	mfs, err := fslibsrv.StartMemFs(name, name)
	if err != nil {
		log.Fatalf("StartMemFs %v\n", err)
	}
	sclnt := procinit.MakeProcClnt(mfs.FsLib, procinit.GetProcLayersMap())
	sclnt.Started(os.Args[1])
	seccomp.LoadFilter()
	mfs.Wait()
	sclnt.Exited(os.Args[1], "OK")
	mfs.FsLib.ShutdownFs(name)
}
