package replraft

import (
	"net"

	raft "go.etcd.io/etcd/raft/v3"

	"sigmaos/config"
	"sigmaos/repl"
	replproto "sigmaos/repl/proto"
)

type RaftReplServer struct {
	storage *raft.MemoryStorage
	node    *RaftNode
	clerk   *Clerk
}

func MakeRaftReplServer(scfg *config.ProcEnv, id int, peerAddrs []string, l net.Listener, init bool, apply repl.Tapplyf) *RaftReplServer {
	srv := &RaftReplServer{}
	peers := []raft.Peer{}
	for i := range peerAddrs {
		peers = append(peers, raft.Peer{ID: uint64(i + 1)})
	}
	commitC := make(chan *committedEntries)
	proposeC := make(chan []byte)
	srv.clerk = newClerk(id, commitC, proposeC, apply)
	srv.node = makeRaftNode(scfg, id, peers, peerAddrs, l, init, srv.clerk, commitC, proposeC)
	return srv
}

func (srv *RaftReplServer) Start() {
	go srv.clerk.serve()
}

func (srv *RaftReplServer) Process(req *replproto.ReplOpRequest, rep *replproto.ReplOpReply) error {
	op := &Op{request: req, reply: rep, ch: make(chan struct{})}
	srv.clerk.request(op)
	<-op.ch
	return op.err
}
