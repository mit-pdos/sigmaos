package main

import (
	"log"
	"os"
	"path"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/memfsd"
	"ulambda/npclnt"
	"ulambda/npsrv"
)

func main() {
	if len(os.Args) < 5 {
		log.Fatalf("Usage: %v pid port config-path union-dir-path", os.Args[0])
	}
	port := os.Args[2]
	configPath := os.Args[3]
	unionDirPath := os.Args[4]
	fsl := fslib.MakeFsLib("memfs-replica" + port)
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("%v: no IP %v\n", os.Args[0], err)
	}
	clnt := npclnt.MakeNpClnt()
	config, err := npsrv.ReadReplConfig(configPath, ip+port, fsl, clnt)
	if err != nil {
		log.Fatalf("Couldn't rea repl config: %v\n", err)
	}
	fsd := memfsd.MakeReplicatedFsd(ip+port, true, config)
	name := path.Join(unionDirPath, fsd.GetSrv().MyAddr())
	db.Name(name)
	fs, err := fslibsrv.InitFs(name, fsd, nil)
	if err != nil {
		log.Fatalf("%v: InitFs failed %v\n", os.Args[0], err)
	}
	fs.Started(os.Args[1])
	fsd.Serve()
	fs.ExitFs(name)
}
