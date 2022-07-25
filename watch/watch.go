package watch

import (
	"sync"

	// "github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/lockmap"
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
	pl   *lockmap.PathLock
	sc   *sesscond.SessCond
	nref int
}

func mkWatch(sct *sesscond.SessCondTable, pl *lockmap.PathLock) *Watch {
	w := &Watch{}
	w.pl = pl
	w.sc = sct.MakeSessCond(pl)
	return w
}

// Caller should hold path lock. On return caller has path lock again
func (ws *Watch) Watch(sessid np.Tsession) *np.Err {
	db.DPrintf("WATCH", "Watch '%s'\n", ws.pl.Path())
	err := ws.sc.Wait(sessid)
	if err != nil {
		db.DPrintf("WATCH_ERR", "Watch done waiting '%v' err %v\n", ws.pl.Path(), err)
	}
	return err
}

func (ws *Watch) Wakeup() {
	ws.sc.Broadcast()
}

type WatchTable struct {
	//      deadlock.Mutex
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

// Alloc watch, if doesn't exist allocate one.  Caller must have pl
// locked.
func (wt *WatchTable) allocWatch(pl *lockmap.PathLock) *Watch {
	wt.Lock()
	defer wt.Unlock()

	p := pl.Path()

	db.DPrintf("WATCH", "allocWatch %s\n", p)

	ws, ok := wt.watches[p]
	if !ok {
		db.DPrintf("WATCH", "mkWatch '%s'\n", p)
		ws = mkWatch(wt.sct, pl)
		wt.watches[p] = ws
	}
	ws.nref++ // ensure ws won't be deleted from table
	return ws
}

func (wt *WatchTable) free(ws *Watch) bool {
	wt.Lock()
	defer wt.Unlock()

	del := false
	ws.nref--

	ws1, ok := wt.watches[ws.pl.Path()]
	if !ok {
		// Another thread already deleted the entry
		db.DFatalf("free '%v'\n", ws)
		return del
	}

	if ws != ws1 {
		db.DFatalf("free\n")
	}

	if ws.nref == 0 {
		delete(wt.watches, ws.pl.Path())
		del = true
	}
	return del
}

// Free watch for path. Caller should hold path lock. If no thread is
// using the watch anymore, free the sess cond associated with the
// watch.
func (wt *WatchTable) FreeWatch(ws *Watch) {
	db.DPrintf("WATCH", "FreeWatch '%s'\n", ws.pl.Path())
	del := wt.free(ws)
	if del {
		wt.sct.FreeSessCond(ws.sc)
	}
}

// Caller should have pl locked
func (wt *WatchTable) WaitWatch(pl *lockmap.PathLock, sid np.Tsession) *np.Err {
	ws := wt.allocWatch(pl)
	err := ws.Watch(sid)
	wt.FreeWatch(ws)
	return err
}

// Caller should have pl locked
func (wt *WatchTable) WakeupWatch(pl *lockmap.PathLock) {
	wt.Lock()
	defer wt.Unlock()

	p := pl.Path()

	db.DPrintf("WATCH", "WakeupWatch '%s'\n", p)
	ws, ok := wt.watches[p]
	if !ok {
		return
	}
	ws.Wakeup()
}
