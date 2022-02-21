package replraft

import (
	"fmt"

	"ulambda/repl"
	"ulambda/threadmgr"
)

type RaftConfig struct {
	id        int
	peerAddrs []string
}

func MakeRaftConfig(id int, peerAddrs []string) *RaftConfig {
	rc := &RaftConfig{}
	rc.id = id
	rc.peerAddrs = []string{}
	for _, addr := range peerAddrs {
		rc.peerAddrs = append(rc.peerAddrs, addr)
	}

	return rc
}

func (rc *RaftConfig) MakeServer(tm *threadmgr.ThreadMgr) repl.Server {
	return MakeRaftReplServer(rc.id, rc.peerAddrs, tm)
}

func (rc *RaftConfig) ReplAddr() string {
	return rc.peerAddrs[rc.id-1]
}

func (rc *RaftConfig) String() string {
	return fmt.Sprintf("&{ id:%v peerAddrs:%v }", rc.id, rc.peerAddrs)
}
