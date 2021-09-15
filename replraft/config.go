package replraft

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"ulambda/protsrv"
	"ulambda/repl"
)

type RaftConfig struct {
	id        int
	replAddr  string
	peerAddrs []string
}

func MakeRaftConfig(id int, peerAddrs []string) *RaftConfig {
	rc := &RaftConfig{}
	rc.id = id
	rc.peerAddrs = peerAddrs
	rc.replAddr = ReplicaAddrFromPeerAddr(peerAddrs[id-1])

	return rc
}

func (rc *RaftConfig) MakeServer(fs protsrv.FsServer) repl.Server {
	return MakeRaftReplServer(rc.id, rc.peerAddrs, fs)
}

func (rc *RaftConfig) ReplAddr() string {
	return rc.replAddr
}

func (rc *RaftConfig) String() string {
	return fmt.Sprintf("&{ id:%v peerAddrs:%v }", rc.id, rc.peerAddrs)
}

func ReplicaAddrFromPeerAddr(peerAddr string) string {
	// Compute replica address as peerAddr + 100
	host, port, err := net.SplitHostPort(peerAddr)
	if err != nil {
		log.Fatalf("Error splitting host port: %v", err)
	}
	portI, err := strconv.Atoi(port)
	if err != nil {
		log.Fatalf("Error conv port: %v", err)
	}
	newPort := strconv.Itoa(portI + 1000)

	return host + ":" + newPort
}
