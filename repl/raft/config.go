package raft

import (
	"fmt"
	"net"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/repl"
	sp "sigmaos/sigmap"
)

type RaftConfig struct {
	npc     *dialproxyclnt.DialProxyClnt
	id      int
	peerEPs []*sp.Tendpoint
	l       net.Listener
	ep      *sp.Tendpoint
	init    bool // Is this node part of the initial cluster? Or is it being added to an existing cluster?
	pe      *proc.ProcEnv
}

func NewRaftConfig(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt, id int, addr *sp.Taddr, init bool) *RaftConfig {
	rc := &RaftConfig{
		pe:   pe,
		npc:  npc,
		id:   id,
		init: init,
	}
	ep, l, err := rc.npc.Listen(sp.INTERNAL_EP, addr)
	if err != nil {
		db.DFatalf("Error listen: %v", err)
	}
	rc.l = l
	rc.ep = ep
	return rc
}

func NValidEPs(eps []*sp.Tendpoint) int {
	n := 0
	for _, ep := range eps {
		if ep != nil {
			n += 1
		}
	}
	return n
}

func (rc *RaftConfig) SetPeerEPs(eps []*sp.Tendpoint) {
	rc.peerEPs = eps
}

func (rc *RaftConfig) NewServer(applyf repl.Tapplyf) (repl.Server, error) {
	return NewRaftReplServer(rc.npc, rc.pe, rc.id, rc.peerEPs, rc.l, rc.init, applyf)
}

func (rc *RaftConfig) ReplEP() *sp.Tendpoint {
	return rc.ep
}

func (rc *RaftConfig) String() string {
	return fmt.Sprintf("&{ id:%v peerEPs:%v init:%v }", rc.id, rc.peerEPs, rc.init)
}
