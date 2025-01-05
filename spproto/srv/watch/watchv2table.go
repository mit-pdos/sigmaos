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

// Close fid and free watch for ws.dir, if no more watchers.  Caller
// should have pl for ws.dir locked
func (wt *WatchV2Table) FreeWatch(ws *WatchV2, fid sp.Tfid) {
	if ws.closeFid(fid) {
		db.DPrintf(db.WATCH, "WatchV2Table FreeWatch %v %v", ws, fid)
		wt.Lock()
		defer wt.Unlock()
		delete(wt.watches, ws.dir)
	}
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
