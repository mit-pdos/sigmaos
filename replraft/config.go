package replraft

import (
	"fmt"
	"net"

	db "sigmaos/debug"
	"sigmaos/repl"
	"sigmaos/threadmgr"
)

type RaftConfig struct {
	started   bool // Indicates whether or not a server has already been started using this config.
	id        int
	peerAddrs []string
	l         net.Listener
	init      bool // Is this node part of the initial cluster? Or is it being added to an existing cluster?
}

func MakeRaftConfig(id int, peerAddrs []string, init bool) *RaftConfig {
	rc := &RaftConfig{}
	rc.id = id
	rc.init = init
	rc.peerAddrs = []string{}
	for _, addr := range peerAddrs {
		rc.peerAddrs = append(rc.peerAddrs, addr)
	}
	l, err := net.Listen("tcp", rc.peerAddrs[rc.id-1])
	if err != nil {
		db.DFatalf("Error listen: %v", err)
	}
	rc.l = l
	rc.peerAddrs[rc.id-1] = l.Addr().String()
	return rc
}

func (rc *RaftConfig) UpdatePeerAddrs(new []string) {
	if rc.started {
		db.DFatalf("Update peers for started server")
	}
	rc.peerAddrs = []string{}
	for _, addr := range new {
		rc.peerAddrs = append(rc.peerAddrs, addr)
	}
}

func (rc *RaftConfig) MakeServer(tm *threadmgr.ThreadMgr) repl.Server {
	rc.started = true
	return MakeRaftReplServer(rc.id, rc.peerAddrs, rc.l, rc.init, tm)
}

func (rc *RaftConfig) ReplAddr() string {
	return rc.l.Addr().String()
}

func (rc *RaftConfig) String() string {
	return fmt.Sprintf("&{ id:%v peerAddrs:%v init:%v }", rc.id, rc.peerAddrs, rc.init)
}
