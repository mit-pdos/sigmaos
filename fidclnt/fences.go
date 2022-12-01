package fidclnt

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/path"
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
func (ft *FenceTable) Insert(p string, f np.Tfence) *fcall.Err {
	ft.Lock()
	defer ft.Unlock()

	path := path.Split(p) // cleans up p

	db.DPrintf("FIDCLNT", "Insert fence %v %v\n", path, f)
	ft.fencedDirs[path.String()] = f
	return nil
}

func (ft *FenceTable) Lookup(p path.Path) *np.Tfence {
	ft.Lock()
	defer ft.Unlock()

	for pn, f := range ft.fencedDirs {
		db.DPrintf("FIDCLNT", "Lookup fence %v %v\n", p, f)
		if p.IsParent(path.Split(pn)) {
			return &f
		}
	}
	db.DPrintf("FIDCLNT", "Lookup fence %v: no fence\n", p)
	return np.MakeFenceNull()

}
