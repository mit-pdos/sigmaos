package replraft

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"path"
	"strconv"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/types"
	raft "go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	stats "go.etcd.io/etcd/server/v3/etcdserver/api/v2stats"
	"go.uber.org/zap"

	db "ulambda/debug"
)

const (
	CLUSTER_ID = 0x01
)

type RaftNode struct {
	id            int
	peerAddrs     []string
	done          chan bool
	commit        chan<- [][]byte
	propose       <-chan []byte
	node          raft.Node
	config        *raft.Config
	storage       *raft.MemoryStorage
	transport     *rafthttp.Transport
	confState     *raftpb.ConfState
	snapshotIndex uint64
	appliedIndex  uint64
}

type commit struct {
	data       []byte
	applyDoneC chan struct{}
}

func makeRaftNode(id int, peers []raft.Peer, peerAddrs []string, commit chan<- [][]byte, propose <-chan []byte) *RaftNode {
	node := &RaftNode{}
	node.id = id
	node.peerAddrs = peerAddrs
	node.done = make(chan bool)
	node.commit = commit
	node.propose = propose
	node.storage = raft.NewMemoryStorage()
	node.config = &raft.Config{
		ID:                        uint64(id),
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   node.storage,
		MaxSizePerMsg:             4096,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
	}
	node.start(peers)
	return node
}

func (n *RaftNode) start(peers []raft.Peer) {
	if n.id == 1 {
		n.node = raft.StartNode(n.config, peers[:1])
	} else {
		n.postNodeId()
		n.node = raft.RestartNode(n.config)
	}
	n.transport = &rafthttp.Transport{
		Logger:      zap.NewExample(),
		ID:          types.ID(n.id),
		ClusterID:   CLUSTER_ID,
		Raft:        n,
		ServerStats: stats.NewServerStats("", ""),
		LeaderStats: stats.NewLeaderStats(zap.NewExample(), strconv.Itoa(n.id)),
		ErrorC:      make(chan error),
	}
	n.transport.Start()
	for i := range peers {
		if i+1 != n.id {
			n.transport.AddPeer(types.ID(i+1), []string{"http://" + n.peerAddrs[i]})
		}
	}

	go n.serveRaft()
	go n.serveChannels()
}

func (n *RaftNode) serveRaft() {
	addr := n.peerAddrs[n.id-1]
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Error listen: %v", err)
	}

	db.DLPrintf("REPLRAFT", "Serving raft, listener %v at %v", n.id, l.Addr().String())

	srv := &http.Server{Handler: apiHandler(n)}
	err = srv.Serve(l)
	if err != nil {
		log.Fatalf("Error server: %v", err)
	}

	<-n.done
}

func (n *RaftNode) serveChannels() {
	snap, err := n.storage.Snapshot()
	if err != nil {
		log.Fatalf("Error getting raft storage: %v", err)
	}
	n.confState = &snap.Metadata.ConfState
	n.snapshotIndex = snap.Metadata.Index
	n.appliedIndex = snap.Metadata.Index

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	go n.sendProposals()

	for {
		select {
		case <-ticker.C:
			n.node.Tick()
		case read := <-n.node.Ready():
			if !raft.IsEmptySnap(read.Snapshot) {
				n.publishSnapshot(read.Snapshot)
				// XXX Right now we don't handle/generate snapshots.
				log.Fatalf("Received snapshot!")
			}
			n.storage.Append(read.Entries)
			n.transport.Send(read.Messages)
			n.handleEntries(read.Entries)
			n.node.Advance()
		case err := <-n.transport.ErrorC:
			log.Fatalf("Raft transport error: %v", err)
		}
	}
}

func (n *RaftNode) publishSnapshot(snapshot raftpb.Snapshot) {
	if raft.IsEmptySnap(snapshot) {
		return
	}

	n.confState = &snapshot.Metadata.ConfState
	n.snapshotIndex = snapshot.Metadata.Index
	n.appliedIndex = snapshot.Metadata.Index
}

func (n *RaftNode) sendProposals() {
	for {
		prop := <-n.propose
		n.node.Propose(context.TODO(), prop)
	}
}

func (n *RaftNode) handleEntries(entries []raftpb.Entry) {
	if len(entries) == 0 {
		return
	}

	data := [][]byte{}
	for _, e := range entries {
		switch e.Type {
		case raftpb.EntryNormal:
			if len(e.Data) == 0 {
				break
			}
			data = append(data, e.Data)
		case raftpb.EntryConfChange:
			var change raftpb.ConfChange
			change.Unmarshal(e.Data)
			n.confState = n.node.ApplyConfChange(change)
			switch change.Type {
			case raftpb.ConfChangeAddNode:
				if len(change.Context) > 0 {
					db.DLPrintf("REPLRAFT", "Adding peer %v", string(change.Context))
					n.transport.AddPeer(types.ID(change.NodeID), []string{"http://" + string(change.Context)})
				}
			case raftpb.ConfChangeRemoveNode:
				if change.NodeID == uint64(n.id) {
					log.Fatalf("Node removed")
				}
				n.transport.RemovePeer(types.ID(change.NodeID))
			}
		default:
			log.Fatalf("Unexpected entry type: %v", e.Type)
		}
	}

	n.commit <- data

	n.appliedIndex = entries[len(entries)-1].Index
}

// Send a post request, indicating that the node will join the cluster.
func (n *RaftNode) postNodeId() {
	for i, addr := range n.peerAddrs {
		if i == n.id-1 {
			continue
		}
		mcr := &membershipChangeReq{uint64(n.id), n.peerAddrs[n.id-1]}
		b, err := json.Marshal(mcr)
		if err != nil {
			log.Fatalf("Error Marshal in RaftNode.postNodeID: %v", err)
		}
		_, err = http.Post("http://"+path.Join(addr, membershipPrefix), "application/json; charset=utf-8", bytes.NewReader(b))
		// Only post the node ID to one node
		if err == nil {
			return
		}
	}
	log.Fatalf("Failed to post node ID")
}

func (n *RaftNode) IsIDRemoved(id uint64) bool {
	return false
}

func (n *RaftNode) Process(ctx context.Context, m raftpb.Message) error {
	return n.node.Step(ctx, m)
}

func (n *RaftNode) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	n.node.ReportSnapshot(id, status)
}

func (n *RaftNode) ReportUnreachable(id uint64) {
	n.node.ReportUnreachable(id)
}
