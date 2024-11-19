package protsrv

import (
	db "sigmaos/debug"
	"sigmaos/protsrv/lockmap"
	protsrv_proto "sigmaos/protsrv/proto"
	"sync"
)

type Watch struct {
	pl   *lockmap.PathLock
	fids []*Fid
}

func newWatch(pl *lockmap.PathLock) *Watch {
	w := &Watch{}
	w.pl = pl
	return w
}

func (w *Watch) LockPl() {
	w.pl.Lock()
}

func (w *Watch) UnlockPl() {
	w.pl.Unlock()
}

type WatchTableV2 struct {
	sync.Mutex
	watches map[string]*Watch
}

func newWatchTableV2() *WatchTableV2 {
	wt := &WatchTableV2{}
	wt.watches = make(map[string]*Watch)
	return wt
}

// Caller should have pl locked. Updates the fid's watch to store this
func (wt *WatchTableV2) AllocWatch(pl *lockmap.PathLock, fid *Fid) *Watch {
	wt.Lock()
	defer wt.Unlock()

	p := pl.Path()

	db.DPrintf(db.WATCH_V2, "AllocWatch '%s'\n", p)

	ws, ok := wt.watches[p]
	if !ok {
		ws = newWatch(pl)
		wt.watches[p] = ws
	}
	ws.fids = append(ws.fids, fid)

	watchObj, ok := fid.Pobj().Obj().(*WatchFsObj)
	if !ok {
		db.DFatalf("fid %v is not a watch\n", fid)
	}
	watchObj.watch = ws
	return ws
}


// Free watch for path. Caller should have pl locked
func (wt *WatchTableV2) FreeWatch(ws *Watch, fid *Fid) bool {
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
	wt.addWatchEvent(pl, &protsrv_proto.WatchEvent{
		File: filename,
		Type: protsrv_proto.WatchEventType_REMOVE,
	})
}

// Caller should have pl locked
func (wt *WatchTableV2) AddCreateEvent(pl *lockmap.PathLock, filename string) {
	wt.addWatchEvent(pl, &protsrv_proto.WatchEvent{
		File: filename,
		Type: protsrv_proto.WatchEventType_CREATE,
	})
}

func (wt *WatchTableV2) addWatchEvent(pl *lockmap.PathLock, event *protsrv_proto.WatchEvent) {
	wt.Lock()
	defer wt.Unlock()

	p := pl.Path()

	ws, ok := wt.watches[p]
	if !ok {
		return
	}

	// db.DPrintf(db.WATCH_V2, "AddWatchEvent '%s' '%s' %v\n", p, event, ws.fids)
	
	for _, fid := range ws.fids {
		fid.mu.Lock()
		watchObj, ok := fid.Pobj().Obj().(*WatchFsObj)
		if !ok {
			db.DFatalf("fid %v is not a watch\n", fid)
		}
		watchObj.events = append(watchObj.events, event)
		watchObj.cond.Broadcast()
		fid.mu.Unlock()
	}
}

