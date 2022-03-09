package fid

import (
	"encoding/json"
	"log"

	"ulambda/ctx"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/sesscond"
)

type FidSnapshot struct {
	Path    []string
	Obj     uint64
	IsOpen  bool
	M       np.Tmode
	Vers    np.TQversion
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
	fs.Obj = fid.obj.Inum()
	fs.M = fid.m
	fs.Vers = fid.vers
	fs.CtxSnap = fid.ctx.Snapshot()
	fs.Cursor = fid.cursor
	b, err := json.Marshal(fs)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding fid: %v", err)
	}
	return b
}

func Restore(fn fs.RestoreF, sct *sesscond.SessCondTable, b []byte) *Fid {
	fsnap := MakeFidSnapshot()
	err := json.Unmarshal(b, fsnap)
	if err != nil {
		log.Fatalf("FATAL error unmarshal fid in restore: %v", err)
	}
	fid := &Fid{}
	fid.path = fsnap.Path
	fid.obj = fn(fsnap.Obj)
	fid.isOpen = fsnap.IsOpen
	fid.m = fsnap.M
	fid.vers = fsnap.Vers
	fid.ctx = ctx.Restore(sct, b)
	fid.cursor = fsnap.Cursor
	return fid
}
