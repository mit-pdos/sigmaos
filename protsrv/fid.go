package protsrv

import (
	"sync"

	"ulambda/fid"
	np "ulambda/ninep"
)

type fidTable struct {
	sync.Mutex
	fids map[np.Tfid]*fid.Fid
}

func makeFidTable() *fidTable {
	ft := &fidTable{}
	ft.fids = make(map[np.Tfid]*fid.Fid)
	return ft
}

func (ft *fidTable) Lookup(fid np.Tfid) (*fid.Fid, *np.Err) {
	ft.Lock()
	defer ft.Unlock()
	f, ok := ft.fids[fid]
	if !ok {
		return nil, np.MkErr(np.TErrUnknownfid, f)
	}
	return f, nil
}

func (ft *fidTable) Add(fid np.Tfid, f *fid.Fid) {
	ft.Lock()
	defer ft.Unlock()

	ft.fids[fid] = f
}

func (ft *fidTable) Del(fid np.Tfid) {
	ft.Lock()
	defer ft.Unlock()
	delete(ft.fids, fid)
}

func (ft *fidTable) ClunkOpen() {
	ft.Lock()
	defer ft.Unlock()

	for _, f := range ft.fids {
		o := f.Pobj().Obj()
		if f.IsOpen() { // has the fid been opened?
			o.Close(f.Pobj().Ctx(), f.Mode())
		}
	}
}
