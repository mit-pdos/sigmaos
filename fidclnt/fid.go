package fidclnt

import (
	"fmt"
	"log"
	"runtime/debug"
	"sync"

	np "ulambda/ninep"
	"ulambda/proc"
)

type FidMap struct {
	sync.Mutex
	next np.Tfid
	fids map[np.Tfid]*Path
}

func mkFidMap() *FidMap {
	fm := &FidMap{}
	fm.fids = make(map[np.Tfid]*Path)
	return fm
}

func (fm *FidMap) String() string {
	str := ""
	for k, v := range fm.fids {
		str += fmt.Sprintf("fid %v chan %v\n", k, v)
	}
	return str
}

func (fm *FidMap) allocFid() np.Tfid {
	fm.Lock()
	defer fm.Unlock()

	fid := fm.next
	fm.next += 1
	return fid
}

func (fm *FidMap) lookup(fid np.Tfid) *Path {
	fm.Lock()
	defer fm.Unlock()

	if p, ok := fm.fids[fid]; ok {
		return p
	}
	return nil
}

func (fm *FidMap) insert(fid np.Tfid, path *Path) {
	fm.Lock()
	defer fm.Unlock()

	fm.fids[fid] = path
}

func (fm *FidMap) free(fid np.Tfid) {
	fm.Lock()
	defer fm.Unlock()

	_, ok := fm.fids[fid]
	if !ok {
		debug.PrintStack()
		log.Fatalf("FATAL %v: freeFid: fid %v unknown %v\n", proc.GetName(), fid, fm.fids)
	}
	delete(fm.fids, fid)
}
