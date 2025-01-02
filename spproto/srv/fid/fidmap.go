package fid

import (
	"sync"

	db "sigmaos/debug"

	"sigmaos/api/fs"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/util/freelist"
	"sigmaos/util/syncmap"
)

type FidMap struct {
	fl   *freelist.FreeList[Fid]
	fids *syncmap.SyncMap[sp.Tfid, *Fid]
}

func NewFidMap(fl *freelist.FreeList[Fid]) *FidMap {
	fm := &FidMap{
		fl:   fl,
		fids: syncmap.NewSyncMap[sp.Tfid, *Fid](),
	}
	return fm
}

func (fm *FidMap) NewFid(n string, obj fs.FsObj, dir fs.Dir, ctx fs.CtxI, m sp.Tmode, qid sp.Tqid) *Fid {
	fid := fm.fl.New()
	fid.mu = sync.Mutex{}
	fid.obj = obj
	fid.name = n
	fid.dir = dir
	fid.ctx = ctx
	fid.isOpen = false
	fid.m = m
	fid.qid = qid
	fid.cursor = 0
	return fid
}

func (fm *FidMap) Free(f *Fid) {
	fm.fl.Free(f)
}

func (fm *FidMap) Len() int {
	return fm.fids.Len()
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
		if f.Ctx().ClntId() == cid {
			fids = append(fids, fid)
		}
		return true
	})
	return fids
}
