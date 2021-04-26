package main

import (
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/memfsd"
)

func main() {
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("%v: no IP %v\n", os.Args[0], err)
	}
	fsd := memfsd.MakeFsd(ip + ":0")
	name := memfsd.MEMFS + "-replicas" + "/" + fsd.GetSrv().MyAddr()
	db.Name(name)
	fsl, err := fslib.InitFs(name, fsd, nil)
	if err != nil {
		log.Fatalf("%v: InitFs failed %v\n", os.Args[0], err)
	}
	fsl.Started(os.Args[1])
	fsd.Serve()
	fsl.ExitFs(name)
}
