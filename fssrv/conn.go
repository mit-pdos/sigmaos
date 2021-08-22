package fssrv

import (
	// "net"
	"sync"
	"ulambda/npapi"
)

type ConnTable struct {
	mu    sync.Mutex
	conns map[npapi.NpAPI]bool
}

func MkConnTable() *ConnTable {
	ct := &ConnTable{}
	ct.conns = make(map[npapi.NpAPI]bool)
	return ct
}

func (ct *ConnTable) Add(conn npapi.NpAPI) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.conns[conn] = true
}

func (ct *ConnTable) Del(conn npapi.NpAPI) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	delete(ct.conns, conn)
}
