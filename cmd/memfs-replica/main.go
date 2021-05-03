package main

import (
	"log"
	"os"
	"strings"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/memfsd"
	"ulambda/npsrv"
)

const (
	CONFIG_PATH = "name/memfs-replica-config.txt"
)

func readConfig(path string, myaddr string, fsl *fslib.FsLib) *npsrv.NpServerReplConfig {
	b, err := fsl.ReadFile(path)
	if err != nil {
		log.Fatalf("Error reading config: %v, %v", path, err)
	}
	cfgString := strings.TrimSpace(string(b))
	servers := strings.Split(cfgString, "\n")
	headAddr := servers[0]
	tailAddr := servers[len(servers)-1]
	prevAddr := tailAddr
	nextAddr := headAddr
	for idx, s := range servers {
		if s == myaddr {
			if idx != 0 {
				prevAddr = servers[idx-1]
			}
			if idx != len(servers)-1 {
				nextAddr = servers[idx+1]
			}
		}
	}
	return &npsrv.NpServerReplConfig{headAddr, tailAddr, prevAddr, nextAddr, nil, nil, nil, nil, nil}
}

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("Usage: %v pid port config-path", os.Args[0])
	}
	port := os.Args[2]
	configPath := os.Args[3]
	fsli := fslib.MakeFsLib("memfs-replica" + port)
	ip, err := fsclnt.LocalIP()
	config := readConfig(configPath, ip+port, fsli)
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
