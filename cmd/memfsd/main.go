package main

import (
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslibsrv"
	"ulambda/memfsd"
	"ulambda/perf"
)

func main() {
	if os.Args[2] != "" { // initial memfsd?
		// If we're benchmarking, make a flame graph
		p := perf.MakePerf()
		if len(os.Args) >= 4 {
			pprofPath := os.Args[3]
			p.SetupPprof(pprofPath)
		}
		if len(os.Args) >= 5 {
			utilPath := os.Args[4]
			p.SetupCPUUtil(100, utilPath)
		}
		defer p.Teardown()
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
