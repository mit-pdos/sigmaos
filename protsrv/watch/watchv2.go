package watch

import (
	"bytes"
	"encoding/binary"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/ninep"
	"sigmaos/protsrv/fid"
	"sigmaos/protsrv/lockmap"
	"sigmaos/protsrv/pobj"
	protsrv_proto "sigmaos/protsrv/proto"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sync"

	"google.golang.org/protobuf/proto"
)

func NewFidWatch(ctx fs.CtxI, watch *WatchV2) *fid.Fid {
	fid := fid.NewFid(pobj.NewPobj(nil, watch, ctx), 0, nil)

	watch.perFidState[fid] = &PerFidState{
		events: nil,
		remainingMsg: nil,
		cond: sync.NewCond(&sync.Mutex{}),
	}

	return fid
}

type PerFidState struct {
	events []*protsrv_proto.WatchEvent
	remainingMsg []byte
	cond *sync.Cond
}

// implements FsObj so that watches can be implemented as fids
type WatchV2 struct {
	pl   *lockmap.PathLock
	mu   sync.Mutex
	perFidState map[*fid.Fid]*PerFidState
}

func newWatchV2(pl *lockmap.PathLock) *WatchV2 {
	w := &WatchV2{
		pl: pl,
		mu: sync.Mutex{},
		perFidState: make(map[*fid.Fid]*PerFidState),
	}
	return w
}

func IsWatch(obj fs.FsObj) bool {
	_, ok := obj.(*WatchV2)
	return ok
}

// creates a buffer with as many events as possible, blocking if there are currently no events
func (wo *WatchV2) GetEventBuffer(fid *fid.Fid, maxLength int) ([]byte, *serr.Err) {
	wo.mu.Lock()
	perFidState := wo.perFidState[fid]

	perFidState.cond.L.Lock()
	defer perFidState.cond.L.Unlock()
	if len(perFidState.remainingMsg) > 0 {
		sendSize := min(maxLength, len(perFidState.remainingMsg))
		ret := perFidState.remainingMsg[:sendSize]
		perFidState.remainingMsg = perFidState.remainingMsg[sendSize:]
		return ret, nil
	}

	db.DPrintf(db.WATCH, "WatchV2 GetEventBuffer: waiting for %s\n", wo.pl.Path())
	for wo.perFidState[fid] != nil && len(wo.perFidState[fid].events) == 0 {
		wo.mu.Unlock()
		perFidState.cond.Wait()
		wo.mu.Lock()
	}
	db.DPrintf(db.WATCH, "WatchV2 GetEventBuffer: Finished waiting for %s\n", wo.pl.Path())

	if wo.perFidState[fid] != perFidState {
		db.DPrintf(db.WATCH, "WatchV2 GetEventBuffer: perFidState changed after watching\n")
		if wo.perFidState[fid] == nil {
			wo.mu.Unlock()
			return nil, serr.NewErr(serr.TErrClosed, "Watch has been closed")
		}
		wo.mu.Unlock()
		db.DFatalf("perFidState changed unexpectedly\n")
	}
	wo.mu.Unlock()

	db.DPrintf(db.WATCH, "WatchV2 GetEventBuffer: %d events for %s\n", len(perFidState.events), wo.pl.Path())

	msg, err := proto.Marshal(&protsrv_proto.WatchEventList{
		Events: perFidState.events,
	})

	if err != nil {
		db.DFatalf("Error marshalling events: %v\n", err)
	}

	perFidState.events = nil

	sz := uint32(len(msg))
	var buf bytes.Buffer
	err = binary.Write(&buf, binary.LittleEndian, sz)
	if err != nil {
		db.DFatalf("Error writing size: %v\n", err)
	}
	_, err = buf.Write(msg)
	if err != nil {
		db.DFatalf("Error writing message: %v\n", err)
	}

	fullMsg := buf.Bytes()
	sendSize := min(maxLength, len(fullMsg))
	perFidState.remainingMsg = fullMsg[sendSize:]
	return fullMsg[:sendSize], nil
}

func (wo *WatchV2) String() string {
	return wo.pl.Path()
}

func (wo *WatchV2) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	return nil, serr.NewErr(serr.TErrInval, "Cannot call stat on watch")
}

func (wo *WatchV2) Open(ctx fs.CtxI, mode sp.Tmode) (fs.FsObj, *serr.Err) {
	return nil, serr.NewErr(serr.TErrInval, "Cannot open watch")
}

func (wo *WatchV2) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	return serr.NewErr(serr.TErrInval, "Cannot close watch")
}

func (wo *WatchV2) Path() sp.Tpath {
	return sp.NoPath
}

func (wo *WatchV2) Perm() sp.Tperm {
	return ninep.DMREAD
}

func (wo *WatchV2) SetParent(dir fs.Dir) {
	
}

func (wo *WatchV2) Unlink() {

}

func (wo *WatchV2) Parent() fs.Dir {
	return nil
}

func (wo *WatchV2) IsLeased() bool {
	return false
}

func (w *WatchV2) LockPl() {
	w.pl.Lock()
}

func (w *WatchV2) UnlockPl() {
	w.pl.Unlock()
}
