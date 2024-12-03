package raft

import (
	"net"

	raft "go.etcd.io/etcd/raft/v3"

	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/apps/kv/repl"
	replproto "sigmaos/apps/kv/repl/proto"
	sp "sigmaos/sigmap"
)

type RaftReplServer struct {
	node  *RaftNode
	clerk *Clerk
}

func NewRaftReplServer(npc *dialproxyclnt.DialProxyClnt, pe *proc.ProcEnv, id int, peerEPs []*sp.Tendpoint, l net.Listener, init bool, apply repl.Tapplyf) (*RaftReplServer, error) {
	var err error
	srv := &RaftReplServer{}
	peers := []raft.Peer{}
	for i := range peerEPs {
		peers = append(peers, raft.Peer{ID: uint64(i + 1)})
	}
	commitC := make(chan *committedEntries)
	proposeC := make(chan []byte)
	srv.clerk = newClerk(commitC, proposeC, apply)
	srv.node, err = newRaftNode(npc, pe, id+1, peers, peerEPs, l, init, srv.clerk, commitC, proposeC)
	if err != nil {
		return nil, err
	}
	return srv, nil
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
