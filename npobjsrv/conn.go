package npobjsrv

import (
	"sync"
)

type ConnTable struct {
	mu    sync.Mutex
	conns map[*NpConn]bool
}

func MkConnTable() *ConnTable {
	ct := &ConnTable{}
	ct.conns = make(map[*NpConn]bool)
	return ct
}

func (ct *ConnTable) Add(conn *NpConn) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.conns[conn] = true
}

func (ct *ConnTable) Del(conn *NpConn) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	delete(ct.conns, conn)
}

func (ct *ConnTable) IterateFids(fi func(*Fid)) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	for npc, _ := range ct.conns {
		npc.mu.Lock()
		for _, f := range npc.fids {
			fi(f)
		}
		npc.mu.Unlock()
	}
}
