package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"ulambda/groupmgr"
	"ulambda/replica"
	"ulambda/replraft"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %v baseAddr", os.Args[0])
	}
	args := os.Args[1:]

	id, err := strconv.Atoi(os.Getenv(groupmgr.GROUPIDX))
	if err != nil {
		log.Fatalf("id conversion error: %v", err)
	}
	// Raft expects ids to be 1-indexed, but groupmgr 0-indexes them.
	id = id + 1

	baseAddr := args[0]

	// Find the base port of the replica group.
	idx := strings.Index(baseAddr, ":")
	if idx < 0 {
		log.Fatalf("FATAL Invalid base addr: %v", baseAddr)
	}
	host := baseAddr[:idx]
	basePort, err := strconv.Atoi(baseAddr[idx+1:])
	if err != nil {
		log.Fatalf("Invalid port num: %v", err)
	}

	// generate the list of peer addresses.
	peers := []string{}
	for i := 0; i < id; i++ {
		peers = append(peers, host+":"+strconv.Itoa(basePort+i))
	}
	log.Printf("peers: %v id: %v", len(peers), id)

	config := replraft.MakeRaftConfig(id, peers)

	name := fmt.Sprintf("memfs-raft-replica-%v", id)

	// Start the replica server
	replica.RunMemfsdReplica(name, config)
}
