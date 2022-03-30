package fid

import (
	"encoding/json"

	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/sesscond"
)

type FidSnapshot struct {
	Path    []string
	Obj     np.Tpath
	IsOpen  bool
	M       np.Tmode
	Qid     np.Tqid
	CtxSnap []byte
	Cursor  int // for directories
}

func MakeFidSnapshot() *FidSnapshot {
	fs := &FidSnapshot{}
	return fs
}

func (fid *Fid) Snapshot() []byte {
	fs := MakeFidSnapshot()
	fs.Path = fid.path
	fs.Obj = fid.obj.Qid().Path
	fs.M = fid.m
	fs.Qid = fid.qid
	fs.CtxSnap = fid.ctx.Snapshot()
	fs.Cursor = fid.cursor
	b, err := json.Marshal(fs)
	if err != nil {
		db.DFatalf("FATAL Error snapshot encoding fid: %v", err)
	}
	return b
}

func Restore(fn fs.RestoreF, sct *sesscond.SessCondTable, b []byte) *Fid {
	fsnap := MakeFidSnapshot()
	err := json.Unmarshal(b, fsnap)
	if err != nil {
		db.DFatalf("FATAL error unmarshal fid in restore: %v", err)
	}
	fid := &Fid{}
	fid.path = fsnap.Path
	fid.obj = fn(fsnap.Obj)
	fid.isOpen = fsnap.IsOpen
	fid.m = fsnap.M
	fid.qid = fsnap.Qid
	fid.ctx = ctx.Restore(sct, b)
	fid.cursor = fsnap.Cursor
	return fid
}
