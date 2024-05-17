package netproxy

import (
	"fmt"
	"net"
	"sync"

	db "sigmaos/debug"
)

type ListenerMap struct {
	sync.Mutex
	ls     map[Tlid]net.Listener
	closed bool
}

func NewListenerMap() *ListenerMap {
	return &ListenerMap{
		ls: make(map[Tlid]net.Listener),
	}
}

func (lm *ListenerMap) CloseListeners() error {
	lm.Lock()
	defer lm.Unlock()

	if lm.closed {
		return fmt.Errorf("Error: close closed netproxysrv")
	}
	lm.closed = true
	lids := make([]Tlid, 0, len(lm.ls))
	for lid, _ := range lm.ls {
		lids = append(lids, lid)
	}
	for _, lid := range lids {
		db.DPrintf(db.NETPROXYSRV, "Close listener %v", lm.ls[lid].Addr())
		lm.closeListenerL(lid)
	}
	return nil
}

// Store a new listener, and assign it an ID
func (lm *ListenerMap) Add(lid Tlid, l net.Listener) error {
	lm.Lock()
	defer lm.Unlock()

	if lm.closed {
		return fmt.Errorf("Error: add listener to closed netproxy conn")
	}

	lm.ls[lid] = l
	return nil
}

// Given an LID, retrieve the associated Listener
func (lm *ListenerMap) Get(lid Tlid) (net.Listener, bool) {
	lm.Lock()
	defer lm.Unlock()

	l, ok := lm.ls[lid]
	return l, ok
}

// Given an LID, retrieve the associated Listener
func (lm *ListenerMap) Close(lid Tlid) bool {
	lm.Lock()
	defer lm.Unlock()

	return lm.closeListenerL(lid)
}

// Caller holds lock
func (lm *ListenerMap) closeListenerL(lid Tlid) bool {
	l, ok := lm.ls[lid]
	if ok {
		delete(lm.ls, lid)
		l.Close()
	}
	return ok
}
