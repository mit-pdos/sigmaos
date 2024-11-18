package fidclnt

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/syncmap"
)

type FidMap struct {
	sync.Mutex
	next sp.Tfid
	fids *syncmap.SyncMap[sp.Tfid, *Channel]
}

func newFidMap() *FidMap {
	fm := &FidMap{}
	fm.fids = syncmap.NewSyncMap[sp.Tfid, *Channel]()
	return fm
}

func (fm *FidMap) String() string {
	str := "["
	fm.fids.Iter(func(k sp.Tfid, v *Channel) bool {
		str += fmt.Sprintf("{%v chan %v},", k, v)
		return true
	})
	return str + "]"
}

func (fm *FidMap) allocFid() sp.Tfid {
	fm.Lock()
	defer fm.Unlock()

	fid := fm.next
	fm.next += 1
	return fid
}

func (fm *FidMap) len() int {
	return fm.fids.Len()
}

func (fm *FidMap) lookup(fid sp.Tfid) *Channel {
	v, _ := fm.fids.Lookup(fid)
	return v
}

func (fm *FidMap) insert(fid sp.Tfid, ch *Channel) {
	fm.fids.Insert(fid, ch)
}

func (fm *FidMap) free(fid sp.Tfid) {
	fm.fids.Delete(fid)
}

func (fm *FidMap) disconnect(fid0 sp.Tfid) {
	ch0, ok := fm.fids.Lookup(fid0)
	if !ok {
		db.DFatalf("disconnect: fid %v unknown\n", fid0)
	}
	db.DPrintf(db.CRASH, "fid disconnect fid %v ch0 %v\n", fid0, ch0)
	fm.fids.Iter(func(fid sp.Tfid, ch *Channel) bool {
		if fid != fid0 && ch.pc == ch0.pc {
			db.DPrintf(db.CRASH, "fid disconnect fid %v ch %v\n", fid, ch)
			fm.fids.UpdateL(fid, nil)
		}
		return true
	})
	fm.fids.Update(fid0, nil)
}
