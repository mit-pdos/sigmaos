package watch

import (
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	protsrv_proto "sigmaos/spproto/srv/proto"
	"sync"
)

type WatchTable struct {
	sync.Mutex
	watches map[sp.Tpath]*Watch
}

func NewWatchTable() *WatchTable {
	wt := &WatchTable{}
	wt.watches = make(map[sp.Tpath]*Watch)
	return wt
}

// Allocate watch for dir. Caller should have acquire pathlock for dir
func (wt *WatchTable) AllocWatch(dir sp.Tpath) *Watch {
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

func (wt *WatchTable) lookupWatch(dir sp.Tpath) (*Watch, bool) {
	wt.Lock()
	defer wt.Unlock()
	ws, ok := wt.watches[dir]
	return ws, ok
}

// Close fid and free watch for ws.dir, if no more watchers.  Caller
// should have acquired pathlock for ws.dir
func (wt *WatchTable) CloseWatcher(ws *Watch, fid sp.Tfid) {
	if ws.closeFid(fid) {
		db.DPrintf(db.WATCH, "WatchTable CloseWatcher %v for %v", fid, ws.dir)
		wt.Lock()
		defer wt.Unlock()
		delete(wt.watches, ws.dir)
	}
}

// Caller should have acquire pathlock for ws.dir
func (wt *WatchTable) AddRemoveEvent(dir sp.Tpath, filename string) {
	if ws, ok := wt.lookupWatch(dir); ok {
		ws.addEvent(&protsrv_proto.WatchEvent{
			File: filename,
			Type: protsrv_proto.WatchEventType_REMOVE,
		})
	}
}

// Caller should have acquire pathlock for ws.dir
func (wt *WatchTable) AddCreateEvent(dir sp.Tpath, filename string) {
	if ws, ok := wt.lookupWatch(dir); ok {
		ws.addEvent(&protsrv_proto.WatchEvent{
			File: filename,
			Type: protsrv_proto.WatchEventType_CREATE,
		})
	}
}
