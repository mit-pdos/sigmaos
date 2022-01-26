package watch

import (
	"log"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	np "ulambda/ninep"
	"ulambda/sesscond"
)

type Watch struct {
	sync.Mutex
	//deadlock.Mutex
	sc   *sesscond.SessCond
	path string // the key in WatchTable
	nref int    // updated under table lock
}

func mkWatch(sct *sesscond.SessCondTable, path string) *Watch {
	w := &Watch{}
	w.path = path
	w.sc = sct.MakeSessCond(&w.Mutex)
	return w
}

// Caller should hold ws lock on return caller has ws lock again
func (ws *Watch) Watch(sessid np.Tsession) error {
	err := ws.sc.Wait(sessid)
	if err != nil {
		log.Printf("Watch done waiting %v p '%v' err %v\n", ws, ws.path, err)
	}
	return err
}

func (ws *Watch) WakeupWatchL() {
	ws.sc.Broadcast()
}

type WatchTable struct {
	//	deadlock.Mutex
	sync.Mutex
	watches map[string]*Watch
	sct     *sesscond.SessCondTable
}

func MkWatchTable(sct *sesscond.SessCondTable) *WatchTable {
	wt := &WatchTable{}
	wt.sct = sct
	wt.watches = make(map[string]*Watch)
	return wt
}

func (wt *WatchTable) allocWatch(path []string) *Watch {
	p := np.Join(path)

	wt.Lock()
	defer wt.Unlock()

	ws, ok := wt.watches[p]
	if !ok {
		// log.Printf("allocWatch: mkWatch %v\n", path)
		ws = mkWatch(wt.sct, p)
		wt.watches[p] = ws
	}
	ws.nref++ // ensure ws won't be deleted from table
	return ws
}

// Returns locked Watch
// XXX Normalize paths (e.g., delete extra /) so that matches
// work for equivalent paths
func (wt *WatchTable) WatchLookupL(path []string) *Watch {
	ws := wt.allocWatch(path)
	ws.Lock()
	return ws
}

func (wt *WatchTable) release(ws *Watch) bool {
	wt.Lock()
	defer wt.Unlock()

	del := false
	ws.nref--

	ws1, ok := wt.watches[ws.path]
	if !ok {
		// Another thread already deleted the entry
		log.Fatalf("release %v\n", ws)
		return del
	}

	if ws != ws1 {
		log.Fatalf("Release\n")
	}

	if ws.nref == 0 {
		delete(wt.watches, ws.path)
		del = true
	}
	return del
}

// Release watch for path. Caller should have watch locked through
// WatchLookupL().  If no thread is using the watch anymore, free the
// sess cond associated with the watch.
func (wt *WatchTable) Release(ws *Watch) {
	ws.Unlock()
	del := wt.release(ws)
	if del {
		wt.sct.FreeSessCond(ws.sc)
	}
}
