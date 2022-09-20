package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	db "sigmaos/debug"
	"sigmaos/replica"
	"sigmaos/replraft"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v baseAddr", os.Args[0])
	}
	args := os.Args[1:]

	id, err := strconv.Atoi("INVALID")
	if err != nil {
		db.DFatalf("id conversion error: %v", err)
	}
	// Raft expects ids to be 1-indexed, but groupmgr 0-indexes them.
	id = id + 1

	baseAddr := args[0]

	// Find the base port of the replica group.
	idx := strings.Index(baseAddr, ":")
	if idx < 0 {
		db.DFatalf("Invalid base addr: %v", baseAddr)
	}
	host := baseAddr[:idx]
	basePort, err := strconv.Atoi(baseAddr[idx+1:])
	if err != nil {
		db.DFatalf("Invalid port num: %v", err)
	}

	// generate the list of peer addresses.
	peers := []string{}
	for i := 0; i < id; i++ {
		peers = append(peers, host+":"+strconv.Itoa(basePort+i))
	}
	log.Printf("peers: %v id: %v", len(peers), id)

	config := replraft.MakeRaftConfig(id, peers, true)

	name := fmt.Sprintf("memfs-raft-replica-%v", id)

	// Start the replica server
	replica.RunMemfsdReplica(name, config)
}
