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
		return nil, serr.NewErr(serr.TErrUnknownfid, fid)
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

type fidEntry struct {
	fid sp.Tfid
	f   *fid.Fid
}

func (ft *fidTable) ClientFids(cid sp.TclntId) []*fidEntry {
	ft.Lock()
	defer ft.Unlock()

	fids := make([]*fidEntry, 0)
	for fid, f := range ft.fids {
		if f.Pobj().Ctx().ClntId() == cid {
			fids = append(fids, &fidEntry{fid, f})
		}
	}
	return fids
}
