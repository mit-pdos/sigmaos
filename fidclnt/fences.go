package fidclnt

import (
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

//
// Keep track of registered fences for a pathname so that caller of
// pathclnt doesn't have to provide them explicitly.
//

type FenceTable struct {
	sync.Mutex
	fencedDirs map[string]*np.Tfence1
}

func MakeFenceTable() *FenceTable {
	ft := &FenceTable{}
	ft.fencedDirs = make(map[string]*np.Tfence1)
	return ft
}

func (ft *FenceTable) Insert(path string, f *np.Tfence1) *np.Err {
	ft.Lock()
	defer ft.Unlock()

	_, ok := ft.fencedDirs[path]
	if !ok {
		ft.fencedDirs[path] = f
		return nil
	}
	return np.MkErr(np.TErrInval, path)
}

func (ft *FenceTable) Lookup(path np.Path) *np.Tfence1 {
	ft.Lock()
	defer ft.Unlock()

	db.DLPrintf("FIDCLNT", "Lookup fence %v\n", path)
	for pn, f := range ft.fencedDirs {
		if path.IsParent(np.Split(pn)) {
			return f
		}
	}
	return &np.Tfence1{}

}
