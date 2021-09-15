package replraft

import (
	"net"

	raft "go.etcd.io/etcd/raft/v3"

	"ulambda/protsrv"
	"ulambda/repl"
)

type RaftReplServer struct {
	storage *raft.MemoryStorage
	node    *RaftNode
	clerk   *Clerk
}

func MakeRaftReplServer(id int, peerAddrs []string, fs protsrv.FsServer) *RaftReplServer {
	srv := &RaftReplServer{}
	peers := []raft.Peer{}
	for i := range peerAddrs {
		peers = append(peers, raft.Peer{ID: uint64(i + 1)})
	}
	commitC := make(chan [][]byte)
	proposeC := make(chan []byte)
	srv.node = makeRaftNode(id, peers, peerAddrs, commitC, proposeC)
	srv.clerk = makeClerk(fs, commitC, proposeC)
	go srv.clerk.serve()
	return srv
}

func (srv *RaftReplServer) Init() {
}

func (srv *RaftReplServer) MakeConn(psrv protsrv.FsServer, conn net.Conn) repl.Conn {
	return MakeRaftReplConn(psrv, conn, srv.clerk)
}
