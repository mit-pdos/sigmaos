package watch

import (
	db "sigmaos/debug"
	"sigmaos/protsrv/lockmap"
	"sync"
)

type WatchV2 struct {
	pl   *lockmap.PathLock
	fids []*FidWatch
}

func newWatchV2(pl *lockmap.PathLock) *WatchV2 {
	w := &WatchV2{}
	w.pl = pl
	return w
}

func (w *WatchV2) LockPl() {
	w.pl.Lock()
}

func (w *WatchV2) UnlockPl() {
	w.pl.Unlock()
}

type WatchTableV2 struct {
	sync.Mutex
	watches map[string]*WatchV2
}

func NewWatchTableV2() *WatchTableV2 {
	wt := &WatchTableV2{}
	wt.watches = make(map[string]*WatchV2)
	return wt
}

// Caller should have pl locked. Updates the fid's watch to store this
func (wt *WatchTableV2) AllocWatch(pl *lockmap.PathLock, fid *FidWatch) *WatchV2 {
	wt.Lock()
	defer wt.Unlock()

	p := pl.Path()

	db.DPrintf(db.WATCH_V2, "AllocWatch '%s'\n", p)

	ws, ok := wt.watches[p]
	if !ok {
		ws = newWatchV2(pl)
		wt.watches[p] = ws
	}
	ws.fids = append(ws.fids, fid)

	fid.watch = ws
	return ws
}


// Free watch for path. Caller should have pl locked
func (wt *WatchTableV2) FreeWatch(ws *WatchV2, fid *FidWatch) bool {
	db.DPrintf(db.WATCH_V2, "FreeWatch '%s'\n", ws.pl.Path())
	wt.Lock()
	defer wt.Unlock()

	del := false
	ix := -1
	for i, f := range ws.fids {
		if f == fid {
			ix = i
			break
		}
	}
	if ix == -1 {
		db.DFatalf("failed to find fid %v in watch %v\n", fid, ws)
		return del
	}
	ws.fids = append(ws.fids[:ix], ws.fids[ix+1:]...)

	ws1, ok := wt.watches[ws.pl.Path()]
	if !ok {
		// Another thread already deleted the entry
		db.DFatalf("free '%v'\n", ws)
		return del
	}

	if ws != ws1 {
		db.DFatalf("free\n")
	}

	if len(ws.fids) == 0 {
		delete(wt.watches, ws.pl.Path())
		del = true
	}
	return del
}

// Caller should have pl locked
func (wt *WatchTableV2) AddRemoveEvent(pl *lockmap.PathLock, filename string) {
	wt.addWatchEvent(pl, "REMOVE " + filename)
}

// Caller should have pl locked
func (wt *WatchTableV2) AddCreateEvent(pl *lockmap.PathLock, filename string) {
	wt.addWatchEvent(pl, "CREATE " + filename)
}

func (wt *WatchTableV2) addWatchEvent(pl *lockmap.PathLock, event string) {
	wt.Lock()
	defer wt.Unlock()

	p := pl.Path()

	ws, ok := wt.watches[p]
	if !ok {
		return
	}

	db.DPrintf(db.WATCH_V2, "AddWatchEvent '%s' '%s' %v\n", p, event, ws.fids)
	
	for _, fid := range ws.fids {
		fid.mu.Lock()
		fid.events = append(fid.events, event)
		fid.cond.Broadcast()
		fid.mu.Unlock()
	}
}

