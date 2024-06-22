package npproxy

import (
	"sync"

	np "sigmaos/sigmap"
)

type fidMap struct {
	sync.Mutex
	fidmap map[np.Tfid]np.Tfid
}

func newFidMap() *fidMap {
	fm := &fidMap{}
	fm.fidmap = make(map[np.Tfid]np.Tfid)
	return fm
}

func (fm *fidMap) mapTo(fid1, fid2 np.Tfid) {
	fm.Lock()
	defer fm.Unlock()

	fm.fidmap[fid1] = fid2
}

func (fm *fidMap) lookup(fid1 np.Tfid) (np.Tfid, bool) {
	fm.Lock()
	defer fm.Unlock()

	fid2, ok := fm.fidmap[fid1]
	return fid2, ok
}

func (fm *fidMap) delete(fid np.Tfid) {
	fm.Lock()
	defer fm.Unlock()

	delete(fm.fidmap, fid)
}
