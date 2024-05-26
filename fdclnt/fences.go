package fdclnt

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

//
// Keep track of registered fences for a pathname so that caller of
// pathclnt doesn't have to provide a fence as argument on each call.
//

type FenceTable struct {
	sync.Mutex
	fencedDirs map[string]sp.Tfence
}

func newFenceTable() *FenceTable {
	ft := &FenceTable{}
	ft.fencedDirs = make(map[string]sp.Tfence)
	return ft
}

// If already exist, just update
func (ft *FenceTable) insert(pn string, f sp.Tfence) error {
	ft.Lock()
	defer ft.Unlock()

	path := path.Split(pn) // cleans up pn

	db.DPrintf(db.FDCLNT, "Insert fence %v %v\n", path, f)
	ft.fencedDirs[path.String()] = f
	return nil
}

func (ft *FenceTable) lookup(pn string) *sp.Tfence {
	p := path.Split(pn) // cleans up pn
	return ft.lookupPath(p)
}

func (ft *FenceTable) lookupPath(p path.Tpathname) *sp.Tfence {
	ft.Lock()
	defer ft.Unlock()

	for pn, f := range ft.fencedDirs {
		db.DPrintf(db.FDCLNT, "Lookup fence %v %v\n", p, f)
		if p.IsParent(path.Split(pn)) {
			return &f
		}
	}
	db.DPrintf(db.FDCLNT, "Lookup fence %v: no fence\n", p)
	return sp.NullFence()
}
