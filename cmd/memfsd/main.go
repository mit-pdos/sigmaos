package main

import (
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslibsrv"
	"ulambda/memfsd"
)

func main() {
	if os.Args[2] != "" { // initial memfsd?
		db.Name("memfsd")
		fsd := memfsd.MakeFsd(os.Args[2])
		fsd.Serve()
	} else { // started as a ulambda
		name := memfsd.MEMFS + "/" + os.Args[1]
		ip, err := fsclnt.LocalIP()
		if err != nil {
			log.Fatalf("%v: no IP %v\n", os.Args[0], err)
		}
		db.Name(name)
		fsd := memfsd.MakeFsd(ip + ":0")
		fsl, err := fslibsrv.InitFs(name, fsd, nil)
		if err != nil {
			log.Fatalf("%v: InitFs failed %v\n", os.Args[0], err)
		}
		fsl.Started(os.Args[1])
		fsd.Serve()
		fsl.ExitFs(name)
	}
}
