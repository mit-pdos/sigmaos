package protsrv

import (
	"fmt"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/ninep"
	"sigmaos/protsrv/pobj"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sync"
)

// implements FsObj so that watches can be implemented as fids
type WatchFsObj struct {
	events []string
	unfinishedEvent string
	cond  *sync.Cond
	watch *Watch
}

func newFidWatch(ctx fs.CtxI) *Fid {
	fid := Fid{
		mu: sync.Mutex{},
		isOpen: false, 
		po: nil, 
		m: 0, 
		qid: nil, 
		cursor: 0,
	}

	fid.po = pobj.NewPobj(nil, &WatchFsObj{
		events: nil,
		unfinishedEvent: "",
		cond:  sync.NewCond(&fid.mu),
		watch: nil,
	}, ctx)

	return &fid
}

func IsWatch(obj fs.FsObj) bool {
	_, ok := obj.(*WatchFsObj)
	return ok
}

// creates a buffer with as many events as possible, blocking if there are currently no events
func (wo *WatchFsObj) GetEventBuffer(maxLength int) []byte {
	wo.cond.L.Lock()
	defer wo.cond.L.Unlock()
	for len(wo.events) == 0 && wo.unfinishedEvent == "" {
		wo.cond.Wait()
	}

	buf := make([]byte, 0)
	offset := uint32(0)

	if wo.unfinishedEvent != "" {
		var offsetDiff int

		buf, wo.unfinishedEvent, offsetDiff = addEvent(buf, maxLength, wo.unfinishedEvent)
		offset += uint32(offsetDiff)
	}

	if wo.unfinishedEvent == "" {
		maxIxReached := -1
		for ix, event := range wo.events {
			db.DPrintf(db.WATCH_V2, "ReadFWatch event %v\n", event)
			eventStr := event + "\n"
			var offsetDiff int
			buf, wo.unfinishedEvent, offsetDiff = addEvent(buf, maxLength - int(offset), eventStr)
			offset += uint32(offsetDiff)

			maxIxReached = ix
			if wo.unfinishedEvent != "" {
				break
			} 
		}

		wo.events = wo.events[maxIxReached + 1:]
	}

	return buf
}

func addEvent(buffer []byte, remainingCapacity int, event string) ([]byte, string, int) {
	if remainingCapacity < len(event) {
		buffer = append(buffer, event[:remainingCapacity]...)
		return buffer, event[remainingCapacity:], remainingCapacity
	}

	buffer = append(buffer, event...)
	return buffer, "", len(event)
}

func (wo *WatchFsObj) Watch() *Watch {
	return wo.watch
}

func (wo *WatchFsObj) String() string {
	return fmt.Sprintf("{events %v unfinishedEvent %s}", wo.events, wo.unfinishedEvent)
}

func (wo *WatchFsObj) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	return nil, serr.NewErr(serr.TErrInval, "Cannot call stat on watch")
}

func (wo *WatchFsObj) Open(ctx fs.CtxI, mode sp.Tmode) (fs.FsObj, *serr.Err) {
	return nil, serr.NewErr(serr.TErrInval, "Cannot open watch")
}

func (wo *WatchFsObj) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	return serr.NewErr(serr.TErrInval, "Cannot close watch")
}

func (wo *WatchFsObj) Path() sp.Tpath {
	return sp.NoPath
}

func (wo *WatchFsObj) Perm() sp.Tperm {
	return ninep.DMREAD
}

func (wo *WatchFsObj) SetParent(dir fs.Dir) {
	
}

func (wo *WatchFsObj) Unlink() {

}

func (wo *WatchFsObj) Parent() fs.Dir {
	return nil
}

func (wo *WatchFsObj) IsLeased() bool {
	return false
}
