package fid

import (
	db "sigmaos/debug"

	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type FidMap struct {
	fids *syncmap.SyncMap[sp.Tfid, *Fid]
}

func NewFidMap() *FidMap {
	fm := &FidMap{syncmap.NewSyncMap[sp.Tfid, *Fid]()}
	return fm
}

func (fm *FidMap) Lookup(fid sp.Tfid) (*Fid, *serr.Err) {
	f, ok := fm.fids.Lookup(fid)
	if !ok {
		return nil, serr.NewErr(serr.TErrUnknownfid, fid)
	}
	return f, nil
}

func (fm *FidMap) LookupDel(fid sp.Tfid) (*Fid, *serr.Err) {
	if f, ok := fm.fids.LookupDelete(fid); !ok {
		return nil, serr.NewErr(serr.TErrUnknownfid, fid)
	} else {
		return f, nil
	}
}

func (fm *FidMap) Insert(fid sp.Tfid, f *Fid) *serr.Err {
	if ok := fm.fids.Insert(fid, f); !ok {
		f1, _ := fm.fids.Lookup(fid)
		db.DPrintf(db.ERROR, "Insert err %v %v %v\n", fid, f, f1)
		return serr.NewErr(serr.TErrExists, fid)
	}
	return nil
}

func (fm *FidMap) Update(fid sp.Tfid, f *Fid) *serr.Err {
	if ok := fm.fids.Update(fid, f); !ok {
		db.DPrintf(db.ERROR, "Update err %v %v\n", fid, f)
		return serr.NewErr(serr.TErrUnknownfid, fid)
	}
	return nil
}

func (fm *FidMap) ClientFids(cid sp.TclntId) []sp.Tfid {
	fids := make([]sp.Tfid, 0)
	fm.fids.Iter(func(fid sp.Tfid, f *Fid) bool {
		if f.Pobj().Ctx().ClntId() == cid {
			fids = append(fids, fid)
		}
		return true
	})
	return fids
}
