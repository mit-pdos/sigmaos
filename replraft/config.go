package replraft

import (
	"fmt"

	"ulambda/protsrv"
	"ulambda/repl"
)

type RaftConfig struct {
	id        int
	peerAddrs []string
}

func MakeRaftConfig(id int, peerAddrs []string) *RaftConfig {
	rc := &RaftConfig{}
	rc.id = id
	rc.peerAddrs = peerAddrs
	return rc
}

func (rc *RaftConfig) MakeServer(fs protsrv.FsServer) repl.Server {
	return MakeRaftReplServer(rc.id, rc.peerAddrs, fs)
}

func (rc *RaftConfig) ReplAddr() string {
	return rc.peerAddrs[rc.id-1]
}

func (rc *RaftConfig) String() string {
	return fmt.Sprintf("&{ id:%v peerAddrs:%v }", rc.id, rc.peerAddrs)
}
