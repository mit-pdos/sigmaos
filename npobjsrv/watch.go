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

type WatchTable struct {
	mu      sync.Mutex
	watches map[string][]*Watch
}

func MkWatchTable() *WatchTable {
	wt := &WatchTable{}
	wt.watches = make(map[string][]*Watch)
	return wt
}

// XXX Normalize paths (e.g., delete extra /) so that matches
// work for equivalent paths
func (wt *WatchTable) Watch(npc *NpConn, path []string) {
	p := np.Join(path)
	wt.mu.Lock()
	_, ok := wt.watches[p]
	if !ok {
		ws := make([]*Watch, 0)
		wt.watches[p] = ws
	}
	w := mkWatch(npc)
	wt.watches[p] = append(wt.watches[p], w)
	db.DLPrintf("WATCH", "Watch %v %v l %v\n", p, w, len(wt.watches[p]))

	wt.mu.Unlock()

	<-w.ch
	db.DLPrintf("WATCH", "Watch done waiting %v %v\n", p, w)
}

// XXX maybe support wakeupOne?
func (wt *WatchTable) WakeupWatch(fn, dir []string) {
	p := np.Join(fn)
	p1 := np.Join(dir)

	// db.DLPrintf("WATCH", "WakeupWatch check for %v\n", p)

	wt.mu.Lock()
	ws, ok := wt.watches[p]
	if ok {
		delete(wt.watches, p)
	}
	ws1, ok1 := wt.watches[p1]
	if ok1 {
		delete(wt.watches, p1)
	}
	wt.mu.Unlock()
	if ok {
		for _, w := range ws {
			db.DLPrintf("WATCH", "WakeupWatch %v %v\n", p, w)
			w.ch <- true
		}
	}
	if ok1 {
		for _, w := range ws1 {
			db.DLPrintf("WATCH", "WakeupWatch %v %v\n", p1, w)
			w.ch <- true
		}
	}
}

// Wakeup threads waiting for a watch on this connection
func (wt *WatchTable) DeleteConn(npc *NpConn) {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	for p, ws := range wt.watches {
		tmp := ws[:0]
		for _, w := range ws {
			if w.npc == npc {
				db.DLPrintf("WATCH", "Delete watch %v %v\n", p, w)
				w.ch <- true
			} else {
				tmp = append(tmp, w)
			}
		}
		wt.watches[p] = tmp
	}
}
