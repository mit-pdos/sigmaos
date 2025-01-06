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

func NewFidWatch(fm *fid.FidMap, ctx fs.CtxI, fid sp.Tfid, watch *Watch) *fid.Fid {
	f := fm.NewFid("fidWatch", watch, nil, ctx, 0, sp.Tqid{})
	watch.newWatcher(fid)
	return f
}

type PerFidState struct {
	fid          sp.Tfid  // the fid for watcher
	dir          sp.Tpath // directory being watched
	events       []*protsrv_proto.WatchEvent
	remainingMsg []byte
	cond         *sync.Cond
	closed       bool
}

// implements FsObj so that watches can be implemented as fids
type Watch struct {
	dir         sp.Tpath // directory being watched
	mu          sync.Mutex
	perFidState map[sp.Tfid]*PerFidState // each watcher has an watch fid
}

func newWatch(dir sp.Tpath) *Watch {
	w := &Watch{
		dir:         dir,
		mu:          sync.Mutex{},
		perFidState: make(map[sp.Tfid]*PerFidState),
	}
	return w
}

func IsWatch(obj fs.FsObj) bool {
	_, ok := obj.(*Watch)
	return ok
}

func (wo *Watch) lookupFidState(fid sp.Tfid) (*PerFidState, bool) {
	wo.mu.Lock()
	defer wo.mu.Unlock()

	f, ok := wo.perFidState[fid]
	return f, ok
}

func (wo *Watch) newWatcher(fid sp.Tfid) {
	wo.mu.Lock()
	defer wo.mu.Unlock()

	wo.perFidState[fid] = &PerFidState{
		fid:          fid,
		dir:          wo.dir,
		events:       nil,
		remainingMsg: nil,
		cond:         sync.NewCond(&sync.Mutex{}),
	}
}

func (wo *Watch) addEvent(event *protsrv_proto.WatchEvent) {
	wo.mu.Lock()
	defer wo.mu.Unlock()

	db.DPrintf(db.WATCH, "%v: AddWatchEvent %v", wo, event)

	for _, perFidState := range wo.perFidState {
		perFidState.addEvent(event)
	}
}

func (wo *Watch) GetEventBuffer(fid sp.Tfid, maxLength int) ([]byte, *serr.Err) {
	perFidState, ok := wo.lookupFidState(fid)
	if !ok {
		db.DPrintf(db.ERROR, "GetEvenBuffer: unknown %v for watching dir %v", fid, wo.dir)
		return nil, nil
	}
	return perFidState.read(maxLength)
}

func (wo *Watch) closeFid(fid sp.Tfid) bool {
	perFidState, ok := wo.lookupFidState(fid)
	if !ok {
		db.DPrintf(db.ERROR, "closeFid: unknown %v for watching dir %v", fid, wo.dir)
		return false
	}
	perFidState.close()

	wo.mu.Lock()
	defer wo.mu.Unlock()

	delete(wo.perFidState, fid)
	if len(wo.perFidState) == 0 {
		return true
	}
	return false
}

func (wo *Watch) Dir() sp.Tpath {
	return wo.dir
}

func (wo *Watch) String() string {
	return wo.dir.String()
}

func (wo *Watch) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
	return nil, serr.NewErr(serr.TErrInval, "Cannot call stat on watch")
}

func (wo *Watch) Open(ctx fs.CtxI, mode sp.Tmode) (fs.FsObj, *serr.Err) {
	return nil, serr.NewErr(serr.TErrInval, "Cannot open watch")
}

func (wo *Watch) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	return serr.NewErr(serr.TErrInval, "Cannot close watch")
}

func (wo *Watch) Path() sp.Tpath {
	return sp.NoPath
}

func (wo *Watch) Perm() sp.Tperm {
	return ninep.DMREAD
}

func (wo *Watch) Unlink() {

}

func (wo *Watch) IsLeased() bool {
	return false
}

func (perFidState *PerFidState) close() {
	perFidState.cond.L.Lock()
	defer perFidState.cond.L.Unlock()

	perFidState.closed = true
	perFidState.cond.Broadcast()
}

func (perFidState *PerFidState) addEvent(event *protsrv_proto.WatchEvent) {
	perFidState.cond.L.Lock()
	defer perFidState.cond.L.Unlock()

	perFidState.events = append(perFidState.events, event)
	perFidState.cond.Broadcast()
}

// read as many events as possible, blocking if there are currently no events
func (perFidState *PerFidState) read(maxLength int) ([]byte, *serr.Err) {
	perFidState.cond.L.Lock()
	defer perFidState.cond.L.Unlock()

	if len(perFidState.remainingMsg) > 0 {
		sendSize := min(maxLength, len(perFidState.remainingMsg))
		ret := perFidState.remainingMsg[:sendSize]
		perFidState.remainingMsg = perFidState.remainingMsg[sendSize:]
		return ret, nil
	}

	if perFidState.closed {
		return nil, nil
	}

	db.DPrintf(db.WATCH, "Watch GetEventBuffer: watcher %v waiting for %v", perFidState.fid, perFidState.dir)
	for len(perFidState.events) == 0 {
		perFidState.cond.Wait()
		if perFidState.closed {
			db.DPrintf(db.WATCH, "Watch GetEventBuffer: watcher %v closed for %v", perFidState.fid, perFidState.dir)
			return nil, nil
		}
	}

	db.DPrintf(db.WATCH, "Watch GetEventBuffer: watcher %v %d events for %v", perFidState.fid, len(perFidState.events), perFidState.dir)

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
