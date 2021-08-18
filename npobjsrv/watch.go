package npobjsrv

import (
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Watch struct {
	npc *NpConn
	ch  chan bool
}

func mkWatch(npc *NpConn) *Watch {
	return &Watch{npc, make(chan bool)}
}

type Watchers struct {
	mu       sync.Mutex
	watchers []*Watch
}

func mkWatchers() *Watchers {
	w := &Watchers{}
	w.watchers = make([]*Watch, 0)
	return w
}

func (w *Watchers) Watch(npc *NpConn) *np.Rerror {
	ws := mkWatch(npc)
	w.watchers = append(w.watchers, ws)
	w.mu.Unlock()
	<-ws.ch
	db.DLPrintf("WATCH", "Watch done waiting %v %v\n", w)

	defer ws.npc.mu.Unlock()
	ws.npc.mu.Lock()
	if npc.closed {
		// XXX Bettter error message?
		return &np.Rerror{"Closed by client"}
	}
	return nil
}

func (ws *Watchers) wakeupWatch() {
	defer ws.mu.Unlock()
	ws.mu.Lock()
	for _, w := range ws.watchers {
		db.DLPrintf("WATCH", "WakeupWatch %v\n", w)
		w.ch <- true
	}
}

func (ws *Watchers) deleteConn(npc *NpConn) {
	defer ws.mu.Unlock()
	ws.mu.Lock()

	tmp := ws.watchers[:0]
	for _, w := range ws.watchers {
		if w.npc == npc {
			db.DLPrintf("WATCH", "Delete watch %v\n", w)
			w.ch <- true
		} else {
			tmp = append(tmp, w)
		}
	}
	ws.watchers = tmp
}

type WatchTable struct {
	mu       sync.Mutex
	watchers map[string]*Watchers
}

func MkWatchTable() *WatchTable {
	wt := &WatchTable{}
	wt.watchers = make(map[string]*Watchers)
	return wt
}

// Returns locked Watchers
// XXX Normalize paths (e.g., delete extra /) so that matches
// work for equivalent paths
func (wt *WatchTable) WatchLookup(dname []string) *Watchers {
	defer wt.mu.Unlock()
	p := np.Join(dname)
	wt.mu.Lock()
	ws, ok := wt.watchers[p]
	if !ok {
		p1 := np.Copy(dname)
		p = np.Join(p1)
		ws = mkWatchers()
		wt.watchers[p] = ws
	}
	ws.mu.Lock()
	return ws
}

// XXX maybe support wakeupOne?
func (wt *WatchTable) WakeupWatch(fn, dir []string) {
	p := np.Join(fn)
	p1 := np.Join(dir)

	db.DLPrintf("WATCH", "WakeupWatch check for %v, %v\n", p, p1)

	wt.mu.Lock()
	ws, ok := wt.watchers[p]
	if ok {
		delete(wt.watchers, p)
	}
	ws1, ok1 := wt.watchers[p1]
	if ok1 {
		delete(wt.watchers, p1)
	}
	wt.mu.Unlock()
	if ok {
		ws.wakeupWatch()
	}
	if ok1 {
		ws1.wakeupWatch()
	}
}

// Wakeup threads waiting for a watch on this connection
func (wt *WatchTable) DeleteConn(npc *NpConn) {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	for _, ws := range wt.watchers {
		ws.deleteConn(npc)
	}
}
