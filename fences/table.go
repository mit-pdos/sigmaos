package fences

import (
	"sync"

	np "ulambda/ninep"
)

// Keep track of fences registered at an fsclnt and at a session in
// fsssrv.

type FenceTable struct {
	sync.Mutex
	fences map[np.Tfenceid]np.Tfence
}

func MakeFenceTable() *FenceTable {
	ft := &FenceTable{}
	ft.fences = make(map[np.Tfenceid]np.Tfence)
	return ft
}

func (ft *FenceTable) Fences() []np.Tfence {
	ft.Lock()
	defer ft.Unlock()

	fences := make([]np.Tfence, 0, len(ft.fences))
	for _, f := range ft.fences {
		fences = append(fences, f)
	}
	return fences
}

func (ft *FenceTable) Insert(f np.Tfence) {
	ft.Lock()
	defer ft.Unlock()

	ft.fences[f.FenceId] = f
}

func (ft *FenceTable) Del(idf np.Tfenceid) error {
	ft.Lock()
	defer ft.Unlock()

	if _, ok := ft.fences[idf]; ok {
		delete(ft.fences, idf)
		return nil
	}
	return np.MkErr(np.TErrUnknownFence, idf)
}

func (ft *FenceTable) Present(idf np.Tfenceid) bool {
	ft.Lock()
	defer ft.Unlock()

	_, ok := ft.fences[idf]
	return ok
}
