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

func (ft *fidTable) LookupDel(fid sp.Tfid) (*fid.Fid, *serr.Err) {
	ft.Lock()
	defer ft.Unlock()
	f, ok := ft.fids[fid]
	if !ok {
		return nil, serr.NewErr(serr.TErrUnknownfid, fid)
	}
	delete(ft.fids, fid)
	return f, nil
}

func (ft *fidTable) Insert(fid sp.Tfid, f *fid.Fid) {
	ft.Lock()
	defer ft.Unlock()

	ft.fids[fid] = f
}

func (ft *fidTable) ClientFids(cid sp.TclntId) []sp.Tfid {
	ft.Lock()
	defer ft.Unlock()

	fids := make([]sp.Tfid, 0)
	for fid, f := range ft.fids {
		if f.Pobj().Ctx().ClntId() == cid {
			fids = append(fids, fid)
		}
	}
	return fids
}
