package fid

import (
	"encoding/json"

	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/sesscond"
)

type PobjSnapshot struct {
	Path    []string
	Obj     np.Tpath
	CtxSnap []byte
}

func MakePobjSnapshot() *PobjSnapshot {
	po := &PobjSnapshot{}
	return po
}

func (po *Pobj) Snapshot() []byte {
	ps := MakePobjSnapshot()
	ps.Path = po.Path()
	ps.Obj = po.Obj().Path()
	ps.CtxSnap = po.Ctx().Snapshot()
	b, err := json.Marshal(ps)
	if err != nil {
		db.DFatalf("Error snapshot encoding fid: %v", err)
	}
	return b
}

func RestorePobj(fn fs.RestoreF, sct *sesscond.SessCondTable, b []byte) *Pobj {
	ps := MakePobjSnapshot()
	err := json.Unmarshal(b, ps)
	if err != nil {
		db.DFatalf("error unmarshal fid in restore: %v", err)
	}
	return &Pobj{ps.Path, fn(ps.Obj), ctx.Restore(sct, b)}
}

type FidSnapshot struct {
	IsOpen   bool
	M        np.Tmode
	Qid      np.Tqid
	PobjSnap []byte
	Cursor   int // for directories
}

func MakeFidSnapshot() *FidSnapshot {
	fs := &FidSnapshot{}
	return fs
}

func (fid *Fid) Snapshot() []byte {
	fs := MakeFidSnapshot()
	fs.M = fid.m
	fs.Qid = fid.qid
	fs.Cursor = fid.cursor
	fs.PobjSnap = fid.po.Snapshot()
	b, err := json.Marshal(fs)
	if err != nil {
		db.DFatalf("Error snapshot encoding fid: %v", err)
	}
	return b
}

func Restore(fn fs.RestoreF, sct *sesscond.SessCondTable, b []byte) *Fid {
	fsnap := MakeFidSnapshot()
	err := json.Unmarshal(b, fsnap)
	if err != nil {
		db.DFatalf("error unmarshal fid in restore: %v", err)
	}
	fid := &Fid{}
	fid.po = RestorePobj(fn, sct, fsnap.PobjSnap)
	fid.isOpen = fsnap.IsOpen
	fid.m = fsnap.M
	fid.qid = fsnap.Qid
	fid.cursor = fsnap.Cursor
	return fid
}
