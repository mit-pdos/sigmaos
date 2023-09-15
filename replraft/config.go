package replraft

import (
	"fmt"
	"net"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/repl"
)

type RaftConfig struct {
	id        int
	peerAddrs []string
	l         net.Listener
	init      bool // Is this node part of the initial cluster? Or is it being added to an existing cluster?
	pcfg      *proc.ProcEnv
}

func MakeRaftConfig(pcfg *proc.ProcEnv, id int, addr string, init bool) *RaftConfig {
	rc := &RaftConfig{}
	rc.id = id
	rc.init = init
	rc.pcfg = pcfg
	l, err := net.Listen("tcp", addr)
	if err != nil {
		db.DFatalf("Error listen: %v", err)
	}
	rc.l = l
	return rc
}

func NValidAddr(peerAddrs []string) int {
	n := 0
	for _, a := range peerAddrs {
		if a != "" {
			n += 1
		}
	}
	return n
}

func (rc *RaftConfig) SetPeerAddrs(new []string) {
	rc.peerAddrs = new
}

func (rc *RaftConfig) MakeServer(applyf repl.Tapplyf) (repl.Server, error) {
	return MakeRaftReplServer(rc.pcfg, rc.id, rc.peerAddrs, rc.l, rc.init, applyf)
}

func (rc *RaftConfig) ReplAddr() string {
	return rc.l.Addr().String()
}

func (rc *RaftConfig) String() string {
	return fmt.Sprintf("&{ id:%v peerAddrs:%v init:%v }", rc.id, rc.peerAddrs, rc.init)
}
