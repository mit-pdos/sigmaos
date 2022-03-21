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
	fencedDirs map[string]np.Tfence1
}

func MakeFenceTable() *FenceTable {
	ft := &FenceTable{}
	ft.fencedDirs = make(map[string]np.Tfence1)
	return ft
}

// if already exist, just update
func (ft *FenceTable) Insert(p string, f np.Tfence1) *np.Err {
	ft.Lock()
	defer ft.Unlock()

	path := np.Split(p) // cleans up p

	db.DLPrintf("FIDCLNT", "Insert fence %v %v\n", path, f)
	ft.fencedDirs[path.String()] = f
	return nil
}

func (ft *FenceTable) Lookup(path np.Path) np.Tfence1 {
	ft.Lock()
	defer ft.Unlock()

	for pn, f := range ft.fencedDirs {
		db.DLPrintf("FIDCLNT", "Lookup fence %v %v\n", path, f)
		if path.IsParent(np.Split(pn)) {
			return f
		}
	}
	db.DLPrintf("FIDCLNT", "Lookup fence %v: no fence\n", path)
	return np.Tfence1{}

}
