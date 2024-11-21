package watch

import (
	db "sigmaos/debug"
	"sigmaos/protsrv/fid"
	"sigmaos/protsrv/lockmap"
	protsrv_proto "sigmaos/protsrv/proto"
	"sync"
)

type WatchV2Table struct {
	sync.Mutex
	watches map[string]*WatchV2
}

func NewWatchV2Table() *WatchV2Table {
	wt := &WatchV2Table{}
	wt.watches = make(map[string]*WatchV2)
	return wt
}

// Caller should have pl locked
func (wt *WatchV2Table) AllocWatch(pl *lockmap.PathLock) *WatchV2 {
	wt.Lock()
	defer wt.Unlock()

	p := pl.Path()

	db.DPrintf(db.WATCH_V2, "AllocWatch '%s'\n", p)

	ws, ok := wt.watches[p]
	if !ok {
		ws = newWatchV2(pl)
		wt.watches[p] = ws
	}

	return ws
}


// Free watch for path. Caller should have pl locked
func (wt *WatchV2Table) FreeWatch(ws *WatchV2, fid *fid.Fid) bool {
	db.DPrintf(db.WATCH_V2, "FreeWatch '%s'\n", ws.pl.Path())
	wt.Lock()
	defer wt.Unlock()

	del := false
	delete(ws.perFidState, fid)

	ws1, ok := wt.watches[ws.pl.Path()]
	if !ok {
		// Another thread already deleted the entry
		db.DFatalf("free '%v'\n", ws)
		return del
	}

	if ws != ws1 {
		db.DFatalf("free\n")
	}

	if len(ws.perFidState) == 0 {
		delete(wt.watches, ws.pl.Path())
		del = true
	}
	return del
}

// Caller should have pl locked
func (wt *WatchV2Table) AddRemoveEvent(pl *lockmap.PathLock, filename string) {
	wt.addWatchEvent(pl, &protsrv_proto.WatchEvent{
		File: filename,
		Type: protsrv_proto.WatchEventType_REMOVE,
	})
}

// Caller should have pl locked
func (wt *WatchV2Table) AddCreateEvent(pl *lockmap.PathLock, filename string) {
	wt.addWatchEvent(pl, &protsrv_proto.WatchEvent{
		File: filename,
		Type: protsrv_proto.WatchEventType_CREATE,
	})
}

func (wt *WatchV2Table) addWatchEvent(pl *lockmap.PathLock, event *protsrv_proto.WatchEvent) {
	wt.Lock()
	defer wt.Unlock()

	p := pl.Path()

	ws, ok := wt.watches[p]
	if !ok {
		return
	}

	db.DPrintf(db.WATCH_V2, "AddWatchEvent '%s' '%v' %v\n", p, event, ws.perFidState)
	
	for _, perFidState := range ws.perFidState {
		perFidState.cond.L.Lock()

		perFidState.events = append(perFidState.events, event)
		perFidState.cond.Broadcast()

		perFidState.cond.L.Unlock()
	}
}

