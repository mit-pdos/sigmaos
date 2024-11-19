package protsrv

import (
	"bytes"
	"encoding/binary"
	"fmt"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/ninep"
	"sigmaos/protsrv/pobj"
	protsrv_proto "sigmaos/protsrv/proto"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sync"

	"google.golang.org/protobuf/proto"
)

// implements FsObj so that watches can be implemented as fids
type WatchFsObj struct {
	events []*protsrv_proto.WatchEvent
	remainingMsg []byte
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
		remainingMsg: nil,
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

	if len(wo.remainingMsg) > 0 {
		sendSize := min(maxLength, len(wo.remainingMsg))
		ret := wo.remainingMsg[:sendSize]
		wo.remainingMsg = wo.remainingMsg[sendSize:]
		return ret
	}

	for len(wo.events) == 0 {
		wo.cond.Wait()
	}

	msg, err := proto.Marshal(&protsrv_proto.WatchEventList{
		Events: wo.events,
	})

	if err != nil {
		db.DFatalf("Error marshalling events: %v\n", err)
		return nil
	}

	wo.events = nil

	sz := uint32(len(msg))
	var buf bytes.Buffer
	err = binary.Write(&buf, binary.LittleEndian, sz)
	if err != nil {
		db.DFatalf("Error writing size: %v\n", err)
		return nil
	}
	_, err = buf.Write(msg)
	if err != nil {
		db.DFatalf("Error writing message: %v\n", err)
		return nil
	}

	fullMsg := buf.Bytes()
	sendSize := min(maxLength, len(fullMsg))
	wo.remainingMsg = fullMsg[sendSize:]
	return fullMsg[:sendSize]
}

func (wo *WatchFsObj) Watch() *Watch {
	return wo.watch
}

func (wo *WatchFsObj) String() string {
	return fmt.Sprintf("{events %v remainingMsg sz %d}", wo.events, len(wo.remainingMsg))
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
