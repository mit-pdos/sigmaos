package fidclnt

import (
	"sync"

	db "sigmaos/debug"
	np "sigmaos/sigmap"
)

//
// Keep track of registered fences for a pathname so that caller of
// pathclnt doesn't have to provide a fence as argument on each call.
//

type FenceTable struct {
	sync.Mutex
	fencedDirs map[string]np.Tfence
}

func MakeFenceTable() *FenceTable {
	ft := &FenceTable{}
	ft.fencedDirs = make(map[string]np.Tfence)
	return ft
}

// If already exist, just update
func (ft *FenceTable) Insert(p string, f np.Tfence) *np.Err {
	ft.Lock()
	defer ft.Unlock()

	path := np.Split(p) // cleans up p

	db.DPrintf("FIDCLNT", "Insert fence %v %v\n", path, f)
	ft.fencedDirs[path.String()] = f
	return nil
}

func (ft *FenceTable) Lookup(path np.Path) *np.Tfence {
	ft.Lock()
	defer ft.Unlock()

	for pn, f := range ft.fencedDirs {
		db.DPrintf("FIDCLNT", "Lookup fence %v %v\n", path, f)
		if path.IsParent(np.Split(pn)) {
			return &f
		}
	}
	db.DPrintf("FIDCLNT", "Lookup fence %v: no fence\n", path)
	return np.MakeFenceNull()

}
