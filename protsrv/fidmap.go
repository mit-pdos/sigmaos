package protsrv

import (
	db "sigmaos/debug"

	"sigmaos/fid"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type fidMap struct {
	fids *syncmap.SyncMap[sp.Tfid, *fid.Fid]
}

func newFidMap() *fidMap {
	fm := &fidMap{syncmap.NewSyncMap[sp.Tfid, *fid.Fid]()}
	return fm
}

func (fm *fidMap) Lookup(fid sp.Tfid) (*fid.Fid, *serr.Err) {
	f, ok := fm.fids.Lookup(fid)
	if !ok {
		return nil, serr.NewErr(serr.TErrUnknownfid, fid)
	}
	return f, nil
}

func (fm *fidMap) LookupDel(fid sp.Tfid) (*fid.Fid, *serr.Err) {
	if f, ok := fm.fids.LookupDelete(fid); !ok {
		return nil, serr.NewErr(serr.TErrUnknownfid, fid)
	} else {
		return f, nil
	}
}

func (fm *fidMap) Insert(fid sp.Tfid, f *fid.Fid) *serr.Err {
	if ok := fm.fids.Insert(fid, f); !ok {
		f1, _ := fm.fids.Lookup(fid)
		db.DPrintf(db.ERROR, "Insert err %v %v %v\n", fid, f, f1)
		return serr.NewErr(serr.TErrExists, fid)
	}
	return nil
}

func (fm *fidMap) Update(fid sp.Tfid, f *fid.Fid) *serr.Err {
	if ok := fm.fids.Update(fid, f); !ok {
		db.DPrintf(db.ERROR, "Update err %v %v %v\n", fid, f)
		return serr.NewErr(serr.TErrUnknownfid, fid)
	}
	return nil
}

func (fm *fidMap) ClientFids(cid sp.TclntId) []sp.Tfid {
	fids := make([]sp.Tfid, 0)
	fm.fids.Iter(func(fid sp.Tfid, f *fid.Fid) {
		if f.Pobj().Ctx().ClntId() == cid {
			fids = append(fids, fid)
		}
	})
	return fids
}
