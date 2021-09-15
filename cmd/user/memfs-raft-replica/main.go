package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"ulambda/fsclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/replica"
	"ulambda/replraft"
)

func main() {
	if len(os.Args) < 6 {
		log.Fatalf("Usage: %v pid id peerAddrs union-dir-path symlink-path", os.Args[0])
	}
	args := os.Args[1:]

	id, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("id conversion error: %v", err)
	}

	peers := strings.Split(args[2], ",")

	unionDirPath := args[3]
	symlinkPath := args[4]
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("%v: no IP %v\n", args, err)
	}
	srvAddr := ip + ":0"

	config := replraft.MakeRaftConfig(id, peers)

	fsl := fslib.MakeFsLib(fmt.Sprintf("memfs-raft-replica-%v", id))
	fsl.Mkdir(unionDirPath, 0777)
	fsl.SymlinkReplica(peers, symlinkPath, 0777|np.DMTMP|np.DMREPL)

	// Start the replica server
	r := replica.MakeMemfsdReplica(args, srvAddr, unionDirPath, config)
	r.Work()
}
