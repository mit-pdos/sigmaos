package main

import (
	"log"
	"os"

	"ulambda/fsclnt"
	"ulambda/fsetcd"
	"ulambda/fslib"
	"ulambda/memfsd"
	"ulambda/procinit"
)

func main() {
	name := memfsd.MEMFS + "/" + os.Args[1]
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("%v: no IP %v\n", os.Args[0], err)
	}
	fs := fsetcd.MakeFsEtcd(ip + ":0")
	fsl := fslib.MakeFsLib("fsetcd")
	err = fsl.PostService(fs.Addr(), name)
	if err != nil {
		log.Fatalf("PostService %v error: %v", name, err)
	}
	sclnt := procinit.MakeProcClnt(fsl, procinit.GetProcLayersMap())
	sclnt.Started(os.Args[1])
	fs.GetSrv().GetStats().MakeElastic(fsl, os.Args[1])
	fs.Serve()
	fs.GetSrv().GetStats().Done()
	sclnt.Exited(os.Args[1])
	err = fsl.Remove(name)
	if err != nil {
		log.Fatalf("Error Remove: %v", err)
	}
}
