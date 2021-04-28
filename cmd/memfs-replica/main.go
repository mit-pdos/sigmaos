package main

import (
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/memfsd"
	"ulambda/npclnt"
	"ulambda/npsrv"
)

func main() {
	if len(os.Args) < 6 {
		log.Fatalf("Usage: %v pid port headAddress tailAddress nextAddress", os.Args[0])
	}
	port := os.Args[2]
	headAddr := os.Args[3]
	tailAddr := os.Args[4]
	prevAddr := os.Args[5]
	nextAddr := os.Args[6]
	clnt := npclnt.MakeNpClnt()
	config := &npsrv.NpServerReplConfig{headAddr, tailAddr, prevAddr, nextAddr, nil, nil, nil, nil, clnt}
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("%v: no IP %v\n", os.Args[0], err)
	}
	fsd := memfsd.MakeReplicatedFsd(ip+port, true, config)
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
