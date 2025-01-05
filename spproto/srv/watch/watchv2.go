package watch

import (
	"bytes"
	"encoding/binary"
	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/ninep"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/spproto/srv/fid"
	protsrv_proto "sigmaos/spproto/srv/proto"
	"sync"

	"google.golang.org/protobuf/proto"
)

func NewFidWatch(fm *fid.FidMap, ctx fs.CtxI, fid sp.Tfid, watch *WatchV2) *fid.Fid {
	f := fm.NewFid("fidWatch", watch, nil, ctx, 0, sp.Tqid{})
	watch.newWatcher(fid, f)
	return f
}

type PerFidState struct {
	fid          *fid.Fid
	dir          sp.Tpath
	events       []*protsrv_proto.WatchEvent
	remainingMsg []byte
	cond         *sync.Cond
}

// implements FsObj so that watches can be implemented as fids
type WatchV2 struct {
	dir         sp.Tpath // directory being watched
	mu          sync.Mutex
	perFidState map[sp.Tfid]*PerFidState // each watcher has an watch fid
}

func newWatchV2(dir sp.Tpath) *WatchV2 {
	w := &WatchV2{
		dir:         dir,
		mu:          sync.Mutex{},
		perFidState: make(map[sp.Tfid]*PerFidState),
	}
	return w
}

func IsWatch(obj fs.FsObj) bool {
	_, ok := obj.(*WatchV2)
	return ok
}

func (wo *WatchV2) lookupFidState(fid sp.Tfid) *PerFidState {
	wo.mu.Lock()
	defer wo.mu.Unlock()

	return wo.perFidState[fid]
}

func (wo *WatchV2) newWatcher(fidn sp.Tfid, f *fid.Fid) {
	wo.mu.Lock()
	defer wo.mu.Unlock()

	wo.perFidState[fidn] = &PerFidState{
		fid:          f,
		dir:          wo.dir,
		events:       nil,
		remainingMsg: nil,
		cond:         sync.NewCond(&sync.Mutex{}),
	}
}

func (wo *WatchV2) addEvent(event *protsrv_proto.WatchEvent) {
	wo.mu.Lock()
	defer wo.mu.Unlock()

	db.DPrintf(db.WATCH, "%v: AddWatchEvent %v", wo, event)

	for _, perFidState := range wo.perFidState {
		perFidState.addEvent(event)
	}
}

// creates a buffer with as many events as possible, blocking if there are currently no events
func (wo *WatchV2) GetEventBuffer(fid sp.Tfid, maxLength int) ([]byte, *serr.Err) {
	perFidState := wo.lookupFidState(fid)
	return perFidState.read(maxLength)
}

func (wo *WatchV2) Dir() sp.Tpath {
	return wo.dir
}

func (wo *WatchV2) String() string {
	return wo.dir.String()
}

func (wo *WatchV2) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
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

func (wo *WatchV2) Unlink() {

}

func (wo *WatchV2) IsLeased() bool {
	return false
}

func (perFidState *PerFidState) addEvent(event *protsrv_proto.WatchEvent) {
	perFidState.cond.L.Lock()
	defer perFidState.cond.L.Unlock()

	perFidState.events = append(perFidState.events, event)
	perFidState.cond.Broadcast()
}

func (perFidState *PerFidState) read(maxLength int) ([]byte, *serr.Err) {
	perFidState.cond.L.Lock()
	defer perFidState.cond.L.Unlock()
	if len(perFidState.remainingMsg) > 0 {
		sendSize := min(maxLength, len(perFidState.remainingMsg))
		ret := perFidState.remainingMsg[:sendSize]
		perFidState.remainingMsg = perFidState.remainingMsg[sendSize:]
		return ret, nil
	}

	db.DPrintf(db.WATCH, "WatchV2 GetEventBuffer: waiting for %v", perFidState.dir)
	for len(perFidState.events) == 0 {
		perFidState.cond.Wait()
	}
	db.DPrintf(db.WATCH, "WatchV2 GetEventBuffer: Finished waiting for %v", perFidState.dir)

	//if wo.perFidState[fid] != perFidState {
	//	db.DPrintf(db.WATCH, "WatchV2 GetEventBuffer: perFidState changed after watching\n")
	//if wo.perFidState[fid] == nil {
	//		return nil, serr.NewErr(serr.TErrClosed, "Watch has been closed")
	//	}
	//	db.DFatalf("perFidState changed unexpectedly\n")
	//}

	db.DPrintf(db.WATCH, "WatchV2 GetEventBuffer: %d events for %v", len(perFidState.events), perFidState.dir)

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
