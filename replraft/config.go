package replraft

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"ulambda/repl"
	"ulambda/threadmgr"
)

const (
	RAFT_PORT_OFFSET = 1000
)

type RaftConfig struct {
	id        int
	replAddr  string
	peerAddrs []string
}

func MakeRaftConfig(id int, peerAddrs []string) *RaftConfig {
	rc := &RaftConfig{}
	rc.id = id
	rc.replAddr = peerAddrs[id-1]
	rc.peerAddrs = []string{}
	for _, addr := range peerAddrs {
		rc.peerAddrs = append(rc.peerAddrs, addRaftPortOffset(addr))
	}

	return rc
}

func (rc *RaftConfig) MakeServer(tm *threadmgr.ThreadMgr) repl.Server {
	return MakeRaftReplServer(rc.id, rc.peerAddrs, tm)
}

func (rc *RaftConfig) ReplAddr() string {
	return rc.replAddr
}

func (rc *RaftConfig) String() string {
	return fmt.Sprintf("&{ id:%v peerAddrs:%v }", rc.id, rc.peerAddrs)
}

func addRaftPortOffset(peerAddr string) string {
	// Compute replica address as peerAddr + RAFT_PORT_OFFSET
	host, port, err := net.SplitHostPort(peerAddr)
	if err != nil {
		log.Fatalf("Error splitting host port: %v", err)
	}
	portI, err := strconv.Atoi(port)
	if err != nil {
		log.Fatalf("Error conv port: %v", err)
	}
	newPort := strconv.Itoa(portI + RAFT_PORT_OFFSET)

	return host + ":" + newPort
}
