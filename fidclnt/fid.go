package fidclnt

import (
	"fmt"
	"runtime/debug"
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type FidMap struct {
	sync.Mutex
	next sp.Tfid
	fids map[sp.Tfid]*Channel
}

func newFidMap() *FidMap {
	fm := &FidMap{}
	fm.fids = make(map[sp.Tfid]*Channel)
	return fm
}

func (fm *FidMap) String() string {
	str := "["
	for k, v := range fm.fids {
		str += fmt.Sprintf("{%v chan %v},", k, v)
	}
	return str + "]"
}

func (fm *FidMap) allocFid() sp.Tfid {
	fm.Lock()
	defer fm.Unlock()

	fid := fm.next
	fm.next += 1
	return fid
}

func (fm *FidMap) lookup(fid sp.Tfid) *Channel {
	fm.Lock()
	defer fm.Unlock()

	if p, ok := fm.fids[fid]; ok {
		return p
	}
	return nil
}

func (fm *FidMap) insert(fid sp.Tfid, path *Channel) {
	fm.Lock()
	defer fm.Unlock()

	fm.fids[fid] = path
}

func (fm *FidMap) free(fid sp.Tfid) {
	fm.Lock()
	defer fm.Unlock()

	_, ok := fm.fids[fid]
	if !ok {
		debug.PrintStack()
		db.DFatalf("freeFid: fid %v unknown\n", fid)
	}
	delete(fm.fids, fid)
}

func (fm *FidMap) disconnect(fid0 sp.Tfid) {
	fm.Lock()
	defer fm.Unlock()

	ch0, ok := fm.fids[fid0]
	if !ok {
		db.DFatalf("disconnect: fid %v unknown\n", fid0)
	}
	db.DPrintf(db.CRASH, "fid disconnect fid %v ch0 %v\n", fid0, ch0)
	for fid, ch := range fm.fids {
		if fid != fid0 && ch.pc == ch0.pc {
			db.DPrintf(db.CRASH, "fid disconnect fid %v ch %v\n", fid, ch)
			fm.fids[fid] = nil
		}
	}
	fm.fids[fid0] = nil
}
