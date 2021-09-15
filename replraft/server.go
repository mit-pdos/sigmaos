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

func MakeRaftReplServer(fs protsrv.FsServer) *RaftReplServer {
	srv := &RaftReplServer{}
	// TODO: get real values
	var id int = 1
	var peers []raft.Peer = []raft.Peer{raft.Peer{ID: uint64(id)}}
	var peerAddrs []string = []string{"localhost:80"}
	var commit chan [][]byte = make(chan [][]byte)
	var propose chan []byte = make(chan []byte)
	srv.node = makeRaftNode(id, peers, peerAddrs, commit, propose)
	srv.clerk = makeClerk(fs, commit, propose)
	go srv.clerk.serve()
	return srv
}

func (srv *RaftReplServer) Init() {
}

func (srv *RaftReplServer) MakeConn(psrv protsrv.FsServer, conn net.Conn) repl.Conn {
	return MakeRaftReplConn(psrv, conn, srv.clerk)
}
