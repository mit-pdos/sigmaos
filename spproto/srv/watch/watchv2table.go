package watch

import (
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/spproto/srv/lockmap"
	protsrv_proto "sigmaos/spproto/srv/proto"
	"sync"
)

type WatchV2Table struct {
	sync.Mutex
	watches map[sp.Tpath]*WatchV2
}

func NewWatchV2Table() *WatchV2Table {
	wt := &WatchV2Table{}
	wt.watches = make(map[sp.Tpath]*WatchV2)
	return wt
}

// Caller should have pl for locked
func (wt *WatchV2Table) AllocWatch(pl *lockmap.PathLock) *WatchV2 {
	wt.Lock()
	defer wt.Unlock()

	p := pl.Path()

	db.DPrintf(db.WATCH, "WatchV2Table AllocWatch %v", p)

	ws, ok := wt.watches[p]
	if !ok {
		ws = newWatchV2(p)
		wt.watches[p] = ws
	}

	return ws
}

// Free watch for path. Caller should have pl for ws.dir locked
func (wt *WatchV2Table) FreeWatch(ws *WatchV2, fid sp.Tfid) bool {
	db.DPrintf(db.WATCH, "WatchV2Table FreeWatch %v %v", ws, fid)
	wt.Lock()
	defer wt.Unlock()

	del := false
	delete(ws.perFidState, fid)

	ws1, ok := wt.watches[ws.dir]
	if !ok {
		// Another thread already deleted the entry
		db.DFatalf("free ws %v", ws)
		return del
	}

	if ws != ws1 {
		db.DFatalf("free")
	}

	if len(ws.perFidState) == 0 {
		delete(wt.watches, ws.dir)
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
	ws.addEvent(event)
}
