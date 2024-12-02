package raft

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"go.etcd.io/etcd/raft/v3/raftpb"

	db "sigmaos/debug"
)

const (
	membershipPrefix = "/members"
)

func apiHandler(n *RaftNode) http.Handler {
	mHandler := newMembershipHandler(n)
	mux := http.NewServeMux()
	mux.Handle("/", n.transport.Handler())
	mux.Handle(membershipPrefix, mHandler)
	return mux
}

type membershipChangeReq struct {
	ID uint64
	IP string
}

type membershipHandler struct {
	n *RaftNode
}

func newMembershipHandler(n *RaftNode) http.Handler {
	h := &membershipHandler{}
	h.n = n
	return h
}

func (h membershipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("X-Etcd-Cluster-ID", h.n.transport.ClusterID.String())

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		db.DFatalf("Error ReadAll membershipHandler.ServeHTTP: %v", err)
		http.Error(w, "Error reading request", http.StatusBadRequest)
		return
	}

	var c membershipChangeReq
	if err := json.Unmarshal(b, &c); err != nil {
		db.DFatalf("Error Unmarshal in membershipHandler.ServeHTTP: %v", err)
		http.Error(w, "Error unmarshalling request", http.StatusBadRequest)
		return
	}

	db.DPrintf(db.REPLRAFT, "MembershipChange %v\n", c)

	cc := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: c.ID, Context: []byte(c.IP)}
	h.n.node.ProposeConfChange(context.TODO(), cc)
	// XXX Should perhaps wait until the change is confirmed?
}
