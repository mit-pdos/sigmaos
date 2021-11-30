package watch

import (
	"log"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/protsrv"
)

type Watch struct {
	npc  protsrv.Protsrv
	cond *sync.Cond
}

func mkWatch(npc protsrv.Protsrv, l sync.Locker) *Watch {
	return &Watch{npc, sync.NewCond(l)}
}

type Watchers struct {
	sync.Mutex
	path     string // the key in WatchTable
	nref     int    // updated under table lock
	watchers []*Watch
}

func mkWatchers(path string) *Watchers {
	w := &Watchers{}
	w.path = path
	w.watchers = make([]*Watch, 0)
	return w
}

// Caller should hold ws lock on return caller has ws lock again
func (ws *Watchers) Watch(npc protsrv.Protsrv) *np.Rerror {
	log.Printf("%p: watch wait %v\n", ws, ws.path)

	w := mkWatch(npc, &ws.Mutex)
	ws.watchers = append(ws.watchers, w)
	w.cond.Wait()

	log.Printf("%v: watch done waiting %v\n", ws, ws.path)

	db.DLPrintf("WATCH", "Watch done waiting %v %v\n", ws, ws.path)

	if npc.Closed() {
		// XXX Bettter error message?
		return &np.Rerror{"Closed by client"}
	}
	return nil
}

func (ws *Watchers) WakeupWatchL() {
	dw := false
	if ws.path == "w" {
		dw = true
		log.Printf("WakeupWatchL [%v]\n", ws.path)
	}
	for _, w := range ws.watchers {
		db.DLPrintf("WATCH", "WakeupWatch %v\n", w)
		if dw {
			log.Printf("%p: wakeup one %v\n", ws, ws.path)
		}
		w.cond.Signal()
	}
	ws.watchers = make([]*Watch, 0)
}

func (ws *Watchers) deleteConn(npc protsrv.Protsrv) {
	ws.Lock()
	defer ws.Unlock()

	tmp := ws.watchers[:0]
	for _, w := range ws.watchers {
		if w.npc == npc {
			db.DLPrintf("WATCH", "Delete watch %v\n", w)
			w.cond.Signal()
		} else {
			tmp = append(tmp, w)
		}
	}
	ws.watchers = tmp
}

type WatchTable struct {
	sync.Mutex
	watchers map[string]*Watchers
	locked   bool
}

func MkWatchTable() *WatchTable {
	wt := &WatchTable{}
	wt.watchers = make(map[string]*Watchers)
	return wt
}

// Returns locked Watchers
// XXX Normalize paths (e.g., delete extra /) so that matches
// work for equivalent paths
func (wt *WatchTable) WatchLookupL(path []string) *Watchers {
	p := np.Join(path)

	log.Printf("watchlookupL %p start [%v]\n", wt, p)

	wt.Lock()

	log.Printf("watchlookupL %p locked [%v]\n", wt, p)

	ws, ok := wt.watchers[p]
	if !ok {
		ws = mkWatchers(p)
		wt.watchers[p] = ws
	}
	ws.nref++ /// ensure ws won't be deleted from table

	log.Printf("watchlookupL: try to lock %p [%v]\n", ws, ws)

	wt.Unlock()

	ws.Lock()

	log.Printf("watchlookup1 done %p [%v]\n", ws, ws)
	return ws
}

// Release watchers for path. Caller should have watchers locked
// through WatchLookupL().
func (wt *WatchTable) Release(ws *Watchers) {
	log.Printf("release %p [%v]\n", wt, ws)
	if ws.path == "w" {
		log.Printf("release [%v]\n", ws.path)
	}
	ws.Unlock()

	wt.Lock()
	defer wt.Unlock()

	ws.nref--

	log.Printf("release locked %p %p:%v\n", wt, ws, ws)

	ws1, ok := wt.watchers[ws.path]
	if !ok {
		// Another thread already deleted the entry
		return
	}

	if ws != ws1 {
		log.Fatalf("Release\n")
	}

	if ws.nref == 0 {
		log.Printf("deleted %p:%v\n", ws, ws.path)
		delete(wt.watchers, ws.path)
	}

	log.Printf("release done %p:[%v]\n", ws, ws)
}

// Wakeup threads waiting for a watch on this connection
func (wt *WatchTable) DeleteConn(npc protsrv.Protsrv) {
	log.Printf("delete %p conn %p\n", wt, npc)

	wt.Lock()
	defer wt.Unlock()

	for _, ws := range wt.watchers {
		ws.deleteConn(npc)
	}
	log.Printf("delete conn %p done\n", npc)
}
