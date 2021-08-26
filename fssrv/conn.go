package fssrv

import (
	"sync"
	"ulambda/protsrv"
)

type ConnTable struct {
	mu    sync.Mutex
	conns map[protsrv.Protsrv]bool
}

func MkConnTable() *ConnTable {
	ct := &ConnTable{}
	ct.conns = make(map[protsrv.Protsrv]bool)
	return ct
}

func (ct *ConnTable) Add(conn protsrv.Protsrv) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.conns[conn] = true
}

func (ct *ConnTable) Del(conn protsrv.Protsrv) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	delete(ct.conns, conn)
}
