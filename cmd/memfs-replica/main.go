package main

import (
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/memfsd"
	"ulambda/npclnt"
	"ulambda/npsrv"
)

const (
	CONFIG_PATH = "name/memfs-replica-config.txt"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("Usage: %v pid port config-path", os.Args[0])
	}
	port := os.Args[2]
	configPath := os.Args[3]
	fsl := fslib.MakeFsLib("memfs-replica" + port)
	ip, err := fsclnt.LocalIP()
	clnt := npclnt.MakeNpClnt()
	config := npsrv.ReadReplConfig(configPath, ip+port, fsl, clnt)
	if err != nil {
		log.Fatalf("%v: no IP %v\n", os.Args[0], err)
	}
	fsd := memfsd.MakeReplicatedFsd(ip+port, true, config)
	name := memfsd.MEMFS + "-replicas" + "/" + fsd.GetSrv().MyAddr()
	db.Name(name)
	fs, err := fslibsrv.InitFs(name, fsd, nil)
	if err != nil {
		log.Fatalf("%v: InitFs failed %v\n", os.Args[0], err)
	}
	fs.Started(os.Args[1])
	fsd.Serve()
	fs.ExitFs(name)
}
