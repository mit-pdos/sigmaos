package watch

import (
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
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

// Allocate watch for dir. Caller should have acquire pathlock for dir
func (wt *WatchV2Table) AllocWatch(dir sp.Tpath) *WatchV2 {
	wt.Lock()
	defer wt.Unlock()

	db.DPrintf(db.WATCH, "WatchV2Table AllocWatch %v", dir)

	ws, ok := wt.watches[dir]
	if !ok {
		ws = newWatchV2(dir)
		wt.watches[dir] = ws
	}

	return ws
}

func (wt *WatchV2Table) lookupWatch(dir sp.Tpath) (*WatchV2, bool) {
	wt.Lock()
	defer wt.Unlock()
	ws, ok := wt.watches[dir]
	return ws, ok
}

// Close fid and free watch for ws.dir, if no more watchers.  Caller
// should have acquired pathlock for ws.dir
func (wt *WatchV2Table) CloseWatcher(ws *WatchV2, fid sp.Tfid) {
	if ws.closeFid(fid) {
		db.DPrintf(db.WATCH, "WatchV2Table CloseWatcher %v for %v", fid, ws.dir)
		wt.Lock()
		defer wt.Unlock()
		delete(wt.watches, ws.dir)
	}
}

// Caller should have acquire pathlock for ws.dir
func (wt *WatchV2Table) AddRemoveEvent(dir sp.Tpath, filename string) {
	if ws, ok := wt.lookupWatch(dir); ok {
		ws.addEvent(&protsrv_proto.WatchEvent{
			File: filename,
			Type: protsrv_proto.WatchEventType_REMOVE,
		})
	}
}

// Caller should have acquire pathlock for ws.dir
func (wt *WatchV2Table) AddCreateEvent(dir sp.Tpath, filename string) {
	if ws, ok := wt.lookupWatch(dir); ok {
		ws.addEvent(&protsrv_proto.WatchEvent{
			File: filename,
			Type: protsrv_proto.WatchEventType_CREATE,
		})
	}
}
