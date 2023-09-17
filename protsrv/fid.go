package protsrv

import (
	"sync"

	"sigmaos/fid"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type fidTable struct {
	sync.Mutex
	fids map[sp.Tfid]*fid.Fid
}

func newFidTable() *fidTable {
	ft := &fidTable{}
	ft.fids = make(map[sp.Tfid]*fid.Fid)
	return ft
}

func (ft *fidTable) Lookup(fid sp.Tfid) (*fid.Fid, *serr.Err) {
	ft.Lock()
	defer ft.Unlock()
	f, ok := ft.fids[fid]
	if !ok {
		return nil, serr.NewErr(serr.TErrUnknownfid, f)
	}
	return f, nil
}

func (ft *fidTable) Add(fid sp.Tfid, f *fid.Fid) {
	ft.Lock()
	defer ft.Unlock()

	ft.fids[fid] = f
}

func (ft *fidTable) Del(fid sp.Tfid) {
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
