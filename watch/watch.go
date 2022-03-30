package watch

import (
	"log"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/sesscond"
)

//
// A table of watches so that client can wait until a file is created
// or removed, or a directory changes (see OWATCH).
//
// Servers also use them to locks a pathname before
// manipulating/creating a file or directory.  When a server starts an
// operation it calls WatchLookupL, which allocates an watch in the
// table and locks the watch. Then, it does it work, and releases the
// watch at the end.  If the releasing thread is the last thread using
// the watch, then the thread removes the watch from the table.
// Thread acquire watches in the following order: first the parent
// directory, then the file or child directory.
//
// If a server thread wants to wait on a watch it calls Watch(), which
// atomically puts the server thread on the waiting list for the watch
// and unlocks the watch.  Another server thread may call Wakeup,
// which causes the waiting thread to resume after acquiring the lock
// on the watch.  This locking plan is modeled after condition
// variables to ensure watches don't suffer from lost wakeups.
//

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
func (ws *Watch) Watch(sessid np.Tsession) *np.Err {
	err := ws.sc.Wait(sessid)
	if err != nil {
		db.DPrintf("WATCH_ERR", "Watch done waiting '%v' err %v\n", ws.path, err)
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

func (wt *WatchTable) allocWatch(path np.Path) *Watch {
	p := path.String()

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
func (wt *WatchTable) WatchLookupL(path np.Path) *Watch {
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
