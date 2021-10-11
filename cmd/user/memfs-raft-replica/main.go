package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/replica"
	"ulambda/replraft"
)

func main() {
	if len(os.Args) < 5 {
		log.Fatalf("Usage: %v pid id peerAddrs union-dir-path", os.Args[0])
	}
	args := os.Args[1:]

	id, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("id conversion error: %v", err)
	}

	peers := strings.Split(args[2], ",")

	unionDirPath := args[3]
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("%v: no IP %v\n", args, err)
	}
	srvAddr := ip + ":0"

	config := replraft.MakeRaftConfig(id, peers)

	fsl := fslib.MakeFsLib(fmt.Sprintf("memfs-raft-replica-%v", id))
	fsl.Mkdir(unionDirPath, 0777)

	// Start the replica server
	replica.RunMemfsdReplica(args, srvAddr, unionDirPath, config)
}
