package main

import (
	"log"
	"os"
	"path"
	"strconv"

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
	relayPort := os.Args[2]
	portNum, err := strconv.Atoi(relayPort)
	if err != nil {
		log.Printf("Port must be an integer")
	}
	// Server port is relay port + 100
	srvPort := strconv.Itoa(100 + portNum)
	configPath := os.Args[3]
	unionDirPath := os.Args[4]
	fsl := fslib.MakeFsLib("memfs-replica:" + relayPort)
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("%v: no IP %v\n", os.Args[0], err)
	}
	relayAddr := ip + ":" + relayPort
	srvAddr := ip + ":" + srvPort
	clnt := npclnt.MakeNpClnt()
	config, err := npsrv.ReadReplConfig(configPath, relayAddr, fsl, clnt)
	if err != nil {
		log.Fatalf("Couldn't read repl config: %v\n", err)
	}
	config.UnionDirPath = unionDirPath
	if len(os.Args) == 6 && os.Args[5] == "log-ops" {
		config.LogOps = true
	}
	fsd := memfsd.MakeReplicatedFsd(srvAddr, true, relayAddr, config)
	name := path.Join(unionDirPath, relayAddr)
	db.Name(name)
	fs, err := fslibsrv.InitFs(name, fsd, nil)
	if err != nil {
		log.Fatalf("%v: InitFs failed %v\n", os.Args[0], err)
	}
	fs.Started(os.Args[1])
	fsd.Serve()
	fs.ExitFs(name)
}
