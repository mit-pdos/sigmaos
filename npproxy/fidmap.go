package npproxy

import (
	"sync"

	sp "sigmaos/sigmap"
)

type fidMap struct {
	sync.Mutex
	fidmap map[sp.Tfid]sp.Tfid
}

func newFidMap() *fidMap {
	fm := &fidMap{}
	fm.fidmap = make(map[sp.Tfid]sp.Tfid)
	return fm
}

func (fm *fidMap) mapTo(fid1, fid2 sp.Tfid) {
	fm.Lock()
	defer fm.Unlock()

	fm.fidmap[fid1] = fid2
}

func (fm *fidMap) lookup(fid1 sp.Tfid) (sp.Tfid, bool) {
	fm.Lock()
	defer fm.Unlock()

	fid2, ok := fm.fidmap[fid1]
	return fid2, ok
}

func (fm *fidMap) delete(fid sp.Tfid) {
	fm.Lock()
	defer fm.Unlock()

	delete(fm.fidmap, fid)
}
