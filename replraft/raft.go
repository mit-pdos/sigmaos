package replraft

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	//"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"github.com/ArielSzekely/etcd/server/v3/etcdserver/api/rafthttp"

	"go.etcd.io/etcd/client/pkg/v3/types"
	raft "go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	stats "go.etcd.io/etcd/server/v3/etcdserver/api/v2stats"
	// etcdstorage "go.etcd.io/etcd/server/v3/storage"
	"go.uber.org/zap"

	db "sigmaos/debug"
	"sigmaos/netproxyclnt"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	CLUSTER_ID = 0x01
)

type RaftNode struct {
	id            int
	peerEPs       []*sp.Tendpoint
	done          chan bool
	commit        chan<- *committedEntries
	propose       <-chan []byte
	node          raft.Node
	config        *raft.Config
	storage       *raft.MemoryStorage
	transport     *rafthttp.Transport
	confState     *raftpb.ConfState
	clerk         *Clerk
	snapshotIndex uint64
	appliedIndex  uint64
	currentLeader uint64
	npc           *netproxyclnt.NetProxyClnt
	pe            *proc.ProcEnv
}

type committedEntries struct {
	entries [][]byte
	leader  uint64
}

// etcd numbers nodes start from 1.  0 is not a valid id.
func newRaftNode(npc *netproxyclnt.NetProxyClnt, pe *proc.ProcEnv, id int, peers []raft.Peer, peerEPs []*sp.Tendpoint, l net.Listener, init bool, clerk *Clerk, commit chan<- *committedEntries, propose <-chan []byte) (*RaftNode, error) {
	node := &RaftNode{
		npc:     npc,
		id:      id,
		peerEPs: peerEPs,
		done:    make(chan bool),
		clerk:   clerk,
		commit:  commit,
		propose: propose,
		storage: raft.NewMemoryStorage(),
		pe:      pe,
	}
	node.config = &raft.Config{
		ID:                        uint64(id),
		ElectionTick:              sp.Conf.Raft.ELECT_NTICKS,
		HeartbeatTick:             sp.Conf.Raft.HEARTBEAT_TICKS,
		Storage:                   node.storage,
		MaxSizePerMsg:             4096,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
	}
	db.DPrintf(db.REPLRAFT, "newRaftNode %d peeraddrs %v\n", id, peerEPs)
	if err := node.start(peers, l, init); err != nil {
		return nil, err
	}
	return node, nil
}

func (n *RaftNode) start(peers []raft.Peer, l net.Listener, init bool) error {
	if init {
		n.node = raft.StartNode(n.config, peers)
	} else {
		n.node = raft.RestartNode(n.config)
	}
	// Make sure the logging dir exists
	os.Mkdir("./raftlogs/", 0777)
	logPath := "./raftlogs/" + n.pe.GetPID().String()
	log.Printf("Raft logs being written to: %v", logPath)
	logCfg := zap.NewDevelopmentConfig()
	logCfg.OutputPaths = []string{string(logPath)}
	// Uncomment for noisier logs
	//	logCfg.OutputPaths = []string{"stdout", string(logPath)}
	//	logCfg.ErrorOutputPaths = []string{"stdout"}
	logger, err := logCfg.Build()
	if err != nil {
		return err
	}
	n.transport = &rafthttp.Transport{
		Logger:      logger,
		ID:          types.ID(n.id),
		ClusterID:   CLUSTER_ID,
		Raft:        n,
		ServerStats: stats.NewServerStats("", ""),
		LeaderStats: stats.NewLeaderStats(zap.NewExample(), strconv.Itoa(n.id)),
		ErrorC:      make(chan error),
		DialContextFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
			addr, err := sp.NewTaddrFromString(address, sp.INNER_CONTAINER_IP, n.pe.GetNet())
			if err != nil {
				db.DFatalf("Error parse addr: %v", err)
			}
			ep := sp.NewEndpoint([]*sp.Taddr{addr}, sp.ROOTREALM)
			c, err := n.npc.Dial(ep)
			if err != nil {
				db.DPrintf(db.REPLRAFT, "Error netproxyclnt Dial raft: %v", err)
			}
			return c, err
		},
	}
	n.transport.Start()
	for i, ep := range n.peerEPs {
		if i != n.id-1 && ep != nil {
			n.transport.AddPeer(types.ID(i+1), []string{"http://" + ep.Addrs()[0].IPPort()})
		}
	}
	go n.serveRaft(l)
	go n.serveChannels()
	return nil
}

func (n *RaftNode) serveRaft(l net.Listener) {
	db.DPrintf(db.REPLRAFT, "Serving raft, listener %v at %v", n.id, l.Addr().String())

	srv := &http.Server{Handler: apiHandler(n)}
	err := srv.Serve(l)
	if err != nil {
		db.DFatalf("Error server: %v", err)
	}

	<-n.done
}

func (n *RaftNode) serveChannels() {
	snap, err := n.storage.Snapshot()
	if err != nil {
		db.DFatalf("Error getting raft storage: %v", err)
	}
	n.confState = &snap.Metadata.ConfState
	n.snapshotIndex = snap.Metadata.Index
	n.appliedIndex = snap.Metadata.Index

	ticker := time.NewTicker(sp.Conf.Raft.TICK_INTERVAL)
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
				db.DFatalf("Received snapshot!")
			}
			n.storage.Append(read.Entries)
			n.transport.Send(read.Messages)
			if read.SoftState != nil {
				// If leadership changed, we may need to re-propose ops.
				if n.currentLeader != read.SoftState.Lead {
					db.DPrintf(db.REPLRAFT, "Leadership change %v %v\n", n.currentLeader, read.SoftState.Lead)
					n.currentLeader = read.SoftState.Lead
				}
			}
			n.handleEntries(read.Entries, n.currentLeader)
			n.node.Advance()
		case err := <-n.transport.ErrorC:
			db.DFatalf("Raft transport error: %v", err)
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

func (n *RaftNode) handleEntries(entries []raftpb.Entry, leader uint64) {
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
					db.DPrintf(db.REPLRAFT, "Adding peer %v", string(change.Context))
					n.transport.AddPeer(types.ID(change.NodeID), []string{"http://" + string(change.Context)})
				}
			case raftpb.ConfChangeRemoveNode:
				if change.NodeID == uint64(n.id) {
					db.DFatalf("Node removed")
				}
				db.DPrintf(db.REPLRAFT, "Removing peer %v", string(change.Context))
				n.transport.RemovePeer(types.ID(change.NodeID))
			}
		default:
			db.DFatalf("Unexpected entry type: %v", e.Type)
		}
	}

	n.commit <- &committedEntries{data, leader}

	n.appliedIndex = entries[len(entries)-1].Index
}

// Send a post request, indicating that the node will join the cluster.
// Note: unused for now.
func (n *RaftNode) postNodeId() error {
	db.DPrintf(db.REPLRAFT, "%v: postNodeId eps %v", n.id, n.peerEPs)
	for i, ep := range n.peerEPs {
		if i == n.id-1 {
			continue
		}
		mcr := &membershipChangeReq{uint64(n.id), n.peerEPs[n.id-1].Addrs()[0].IPPort()}
		b, err := json.Marshal(mcr)
		if err != nil {
			db.DFatalf("Error Marshal in RaftNode.postNodeID: %v", err)
		}
		db.DPrintf(db.REPLRAFT, "Invoke Post node ID %d %v\n", i, ep)
		if _, err := http.Post("http://"+path.Join(ep.Addrs()[0].IPPort(), membershipPrefix), "application/json; charset=utf-8", bytes.NewReader(b)); err == nil {
			db.DPrintf(db.REPLRAFT, "Posted node ID %d %v\n", i, ep)
			// Only post the node ID to one node
			return nil
		} else {
			db.DPrintf(db.REPLRAFT, "Error posting node ID %d %v err %v\n", i, ep, err)
		}
	}
	db.DPrintf(db.REPLRAFT, "postNodeId %v unreachable %v\n", n.id, n.peerEPs)
	return serr.NewErr(serr.TErrUnreachable, "no peers")
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
