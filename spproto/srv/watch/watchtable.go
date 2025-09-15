package watch

import (
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/spproto/srv/fid"
	protsrv_proto "sigmaos/spproto/srv/proto"
	"sync"
)

type WatchTable struct {
	sync.Mutex
	watches map[sp.Tuid]*Watch
}

func NewWatchTable() *WatchTable {
	wt := &WatchTable{}
	wt.watches = make(map[sp.Tuid]*Watch)
	return wt
}

// Allocate watch for dir. Caller should have acquire pathlock for dir
func (wt *WatchTable) AllocWatch(dir sp.Tuid) *Watch {
	wt.Lock()
	defer wt.Unlock()

	db.DPrintf(db.WATCH, "WatchTable AllocWatch %v", dir)

	ws, ok := wt.watches[dir]
	if !ok {
		ws = newWatch(dir)
		wt.watches[dir] = ws
	}

	return ws
}

func (wt *WatchTable) lookupWatch(dir sp.Tuid) (*Watch, bool) {
	wt.Lock()
	defer wt.Unlock()
	ws, ok := wt.watches[dir]
	return ws, ok
}

// Close fid and free watch for ws.dir, if no more watchers.  Caller
// should have acquired pathlock for ws.dir
func (wt *WatchTable) CloseWatcher(ws *Watch, fid *fid.Fid) {
	if ws.closeFid(fid) {
		db.DPrintf(db.WATCH, "WatchTable CloseWatcher %v for %v", fid, ws.dir)
		wt.Lock()
		defer wt.Unlock()
		delete(wt.watches, ws.dir)
	}
}

// Caller should have acquire pathlock for ws.dir
func (wt *WatchTable) AddRemoveEvent(dir sp.Tuid, filename string) {
	if ws, ok := wt.lookupWatch(dir); ok {
		ws.addEvent(&protsrv_proto.WatchEvent{
			File: filename,
			Type: protsrv_proto.WatchEventType_REMOVE,
		})
	}
}

// Caller should have acquire pathlock for ws.dir
func (wt *WatchTable) AddCreateEvent(dir sp.Tuid, filename string) {
	if ws, ok := wt.lookupWatch(dir); ok {
		ws.addEvent(&protsrv_proto.WatchEvent{
			File: filename,
			Type: protsrv_proto.WatchEventType_CREATE,
		})
	}
}
