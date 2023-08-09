package replraft

import (
	"net"

	raft "go.etcd.io/etcd/raft/v3"

	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type RaftReplServer struct {
	storage *raft.MemoryStorage
	node    *RaftNode
	clerk   *Clerk
}

func MakeRaftReplServer(id int, peerAddrs []string, l net.Listener, init bool) *RaftReplServer {
	srv := &RaftReplServer{}
	peers := []raft.Peer{}
	for i := range peerAddrs {
		peers = append(peers, raft.Peer{ID: uint64(i + 1)})
	}
	commitC := make(chan *committedEntries)
	proposeC := make(chan []byte)
	srv.clerk = makeClerk(id, commitC, proposeC)
	srv.node = makeRaftNode(id, peers, peerAddrs, l, init, srv.clerk, commitC, proposeC)
	return srv
}

func (srv *RaftReplServer) Start() {
	go srv.clerk.serve()
}

func (srv *RaftReplServer) Process(fc *sessp.FcallMsg) {
	if fc.GetType() == sessp.TTdetach {
		msg := fc.Msg.(*sp.Tdetach)
		msg.PropId = uint32(srv.node.id)
		fc.Msg = msg
	}
	op := &Op{}
	op.request = fc
	op.reply = nil
	srv.clerk.request(op)
}
