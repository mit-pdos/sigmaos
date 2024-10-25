package protsrv

import (
	db "sigmaos/debug"

	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type fidWatchMap struct {
	fidWatches *syncmap.SyncMap[sp.Tfid, *FidWatch]
}

func newFidWatchMap() *fidWatchMap {
	fm := &fidWatchMap{syncmap.NewSyncMap[sp.Tfid, *FidWatch]()}
	return fm
}

func (fm *fidWatchMap) Lookup(fid sp.Tfid) (*FidWatch, *serr.Err) {
	f, ok := fm.fidWatches.Lookup(fid)
	if !ok {
		return nil, serr.NewErr(serr.TErrUnknownfid, fid)
	}
	return f, nil
}

func (fm *fidWatchMap) LookupDel(fid sp.Tfid) (*FidWatch, *serr.Err) {
	if f, ok := fm.fidWatches.LookupDelete(fid); !ok {
		return nil, serr.NewErr(serr.TErrUnknownfid, fid)
	} else {
		return f, nil
	}
}

func (fm *fidWatchMap) Insert(fid sp.Tfid, f *FidWatch) *serr.Err {
	db.DPrintf(db.WATCH_NEW, "Insert fid %v f %v\n", fid, f)
	if ok := fm.fidWatches.Insert(fid, f); !ok {
		f1, _ := fm.fidWatches.Lookup(fid)
		db.DPrintf(db.ERROR, "Insert err %v %v %v\n", fid, f, f1)
		return serr.NewErr(serr.TErrExists, fid)
	}
	return nil
}

func (fm *fidWatchMap) Update(fid sp.Tfid, f *FidWatch) *serr.Err {
	if ok := fm.fidWatches.Update(fid, f); !ok {
		db.DPrintf(db.ERROR, "Update err %v %v\n", fid, f)
		return serr.NewErr(serr.TErrUnknownfid, fid)
	}
	return nil
}

func (fm *fidWatchMap) ClientFids(cid sp.TclntId) []sp.Tfid {
	fids := make([]sp.Tfid, 0)
	fm.fidWatches.Iter(func(fid sp.Tfid, f *FidWatch) {
		if f.Pobj().Ctx().ClntId() == cid {
			fids = append(fids, fid)
		}
	})
	return fids
}
